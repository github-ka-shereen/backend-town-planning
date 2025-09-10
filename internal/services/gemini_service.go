package services

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"town-planning-backend/config"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/genai"
)

type GeminiService struct {
	client      *genai.Client
	cache       map[string]*CachedResponse
	cacheMutex  sync.RWMutex
	rateLimiter *rate.Limiter
}

type CachedResponse struct {
	Data      string
	ExpiresAt time.Time
}

type RetryConfig struct {
	MaxRetries    int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

func NewGeminiService(apiKey string) (*GeminiService, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	service := &GeminiService{
		client:      client,
		cache:       make(map[string]*CachedResponse),
		rateLimiter: rate.NewLimiter(rate.Every(time.Minute), 15), // 15 requests per minute
	}

	// Start background cache cleanup
	service.StartCacheCleanup()

	return service, nil
}

func (g *GeminiService) GenerateContentWithRetry(ctx context.Context, prompt string, config *RetryConfig) (string, error) {
	if config == nil {
		config = &RetryConfig{
			MaxRetries:    3,
			InitialDelay:  time.Second,
			MaxDelay:      time.Minute,
			BackoffFactor: 2.0,
		}
	}

	// Check cache first
	if cached := g.getFromCache(prompt); cached != "" {
		return cached, nil
	}

	// Rate limit check
	if err := g.rateLimiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("rate limit exceeded: %w", err)
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
				// Continue with retry
			}
		}

		result, err := g.generateContent(ctx, prompt)
		if err == nil {
			// Cache successful response
			g.cacheResponse(prompt, result)
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !g.isRetryableError(err) {
			break
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * config.BackoffFactor)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return "", fmt.Errorf("failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

func (g *GeminiService) generateContent(ctx context.Context, prompt string) (string, error) {
	// Log the prompt being sent
	config.Logger.Info("Sending request to Gemini 2.5 Flash",
		zap.String("type", "text"),
		zap.String("prompt", prompt),
	)

	// Build a single-part content with your prompt
	parts := []*genai.Part{
		{Text: prompt},
	}
	contents := []*genai.Content{
		{Parts: parts},
	}

	startTime := time.Now()

	// Call the new Models.GenerateContent API
	resp, err := g.client.Models.GenerateContent(ctx, "gemini-2.5-flash", contents, nil)
	if err != nil {
		config.Logger.Error("Gemini API request failed",
			zap.String("type", "text"),
			zap.String("prompt", prompt),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)),
		)
		return "", err
	}

	responseText := resp.Text()

	// Log successful response
	config.Logger.Info("Received response from Gemini 2.5 Flash",
		zap.String("type", "text"),
		zap.String("prompt", prompt),
		zap.String("response", responseText),
		zap.Duration("duration", time.Since(startTime)),
	)

	return responseText, nil
}

// Updated method for processing documents (including PDFs)
func (g *GeminiService) ProcessDocumentWithPrompt(ctx context.Context, fileBytes []byte, mimeType string, prompt string) (string, error) {
	// Rate limit check
	if err := g.rateLimiter.Wait(ctx); err != nil {
		config.Logger.Error("Rate limit exceeded",
			zap.String("type", "document"),
			zap.String("mimeType", mimeType),
			zap.Error(err),
		)
		return "", fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Log document processing request
	config.Logger.Info("Processing document with Gemini 2.5 Flash",
		zap.String("type", "document"),
		zap.String("mimeType", mimeType),
		zap.Int("fileSize", len(fileBytes)),
		zap.String("prompt", prompt),
	)

	// Create cache key from both file content and prompt
	// cacheKey := g.generateDocumentCacheKey(fileBytes, prompt)
	// if cached := g.getFromCacheByKey(cacheKey); cached != "" {
	// 	return cached, nil
	// }

	// Build parts for multimodal request
	parts := []*genai.Part{
		{Text: prompt}, // The text instructions
		{InlineData: &genai.Blob{
			MIMEType: mimeType,
			Data:     fileBytes,
		}},
	}

	contents := []*genai.Content{
		{Parts: parts},
	}

	resp, err := g.client.Models.GenerateContent(ctx, "gemini-2.5-flash", contents, nil)
	if err != nil {
		config.Logger.Error("Gemini API request failed",
			zap.String("type", "document"),
			zap.String("mimeType", mimeType),
			zap.Error(err),
		)
		return "", err
	}

	result := resp.Text()

	// Cache the result
	// g.cacheResponseByKey(cacheKey, result)

	return result, nil
}

// Keep the original method for backward compatibility
func (g *GeminiService) ProcessDocument(ctx context.Context, file io.Reader, mimeType string) (string, error) {
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		config.Logger.Error("Failed to read document",
			zap.String("type", "document"),
			zap.String("mimeType", mimeType),
			zap.Error(err),
		)
		return "", err
	}

	// Use a default prompt for document processing
	defaultPrompt := "Please extract and summarize the key information from this document."
	return g.ProcessDocumentWithPrompt(ctx, fileBytes, mimeType, defaultPrompt)
}

func (g *GeminiService) isRetryableError(err error) bool {
	// Check for rate limit, quota, or temporary errors
	errStr := err.Error()
	retryableErrors := []string{
		"rate limit",
		"quota exceeded",
		"temporary",
		"timeout",
		"connection",
		"503",
		"429",
		"internal error",
		"service unavailable",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(strings.ToLower(errStr), retryable) {
			return true
		}
	}
	return false
}

func (g *GeminiService) getFromCache(prompt string) string {
	key := g.generateCacheKey(prompt)
	return g.getFromCacheByKey(key)
}

func (g *GeminiService) getFromCacheByKey(key string) string {
	g.cacheMutex.RLock()
	defer g.cacheMutex.RUnlock()

	if cached, exists := g.cache[key]; exists {
		if time.Now().Before(cached.ExpiresAt) {
			return cached.Data
		}
		// Remove expired entry
		delete(g.cache, key)
	}
	return ""
}

func (g *GeminiService) cacheResponse(prompt, response string) {
	key := g.generateCacheKey(prompt)
	g.cacheResponseByKey(key, response)
}

func (g *GeminiService) cacheResponseByKey(key, response string) {
	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	g.cache[key] = &CachedResponse{
		Data:      response,
		ExpiresAt: time.Now().Add(24 * time.Hour), // Cache for 24 hours
	}
}

func (g *GeminiService) generateCacheKey(prompt string) string {
	hash := md5.Sum([]byte(prompt))
	return hex.EncodeToString(hash[:])
}

func (g *GeminiService) generateDocumentCacheKey(fileBytes []byte, prompt string) string {
	// Combine file hash and prompt hash for cache key
	fileHash := md5.Sum(fileBytes)
	promptHash := md5.Sum([]byte(prompt))
	combined := append(fileHash[:], promptHash[:]...)
	finalHash := md5.Sum(combined)
	return hex.EncodeToString(finalHash[:])
}

func (g *GeminiService) StartCacheCleanup() {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			g.cleanupExpiredCache()
		}
	}()
}

func (g *GeminiService) cleanupExpiredCache() {
	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	now := time.Now()
	for key, cached := range g.cache {
		if now.After(cached.ExpiresAt) {
			delete(g.cache, key)
		}
	}
}
