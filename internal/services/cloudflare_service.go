package services

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type CloudflareWorkerAIService struct {
	accountID   string
	apiToken    string
	httpClient  *http.Client
	cache       map[string]*CachedResponse
	cacheMutex  sync.RWMutex
	rateLimiter *rate.Limiter
}

// CloudflareDocumentRequest for direct PDF processing
type CloudflareDocumentRequest struct {
	Prompt   string `json:"prompt"`
	Document string `json:"document"` // base64-encoded PDF
}

// CloudflareTextRequest for text-based models
type CloudflareTextRequest struct {
	Messages  []CloudflareMessage `json:"messages"`
	MaxTokens int                 `json:"max_tokens,omitempty"`
}

type CloudflareMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CloudflareDocumentResponse for document processing
type CloudflareDocumentResponse struct {
	Result struct {
		Response    string `json:"response"`
		Description string `json:"description"`
		Text        string `json:"text"`
	} `json:"result"`
	Success  bool              `json:"success"`
	Errors   []CloudflareError `json:"errors"`
	Messages []string          `json:"messages"`
}

type CloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewCloudflareWorkerAIService(accountID, apiToken string) (*CloudflareWorkerAIService, error) {
	if accountID == "" || apiToken == "" {
		return nil, fmt.Errorf("account ID and API token are required")
	}

	service := &CloudflareWorkerAIService{
		accountID: accountID,
		apiToken:  apiToken,
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // 5 minutes for large PDFs
		},
		cache:       make(map[string]*CachedResponse),
		rateLimiter: rate.NewLimiter(rate.Every(10*time.Second), 5), // Conservative rate limiting
	}

	service.StartCacheCleanup()
	return service, nil
}

// Process document directly without conversion
func (c *CloudflareWorkerAIService) ProcessDocumentWithPrompt(ctx context.Context, fileBytes []byte, mimeType string, prompt string) (string, error) {
	// Rate limit check
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Create cache key
	cacheKey := c.generateDocumentCacheKey(fileBytes, prompt)
	if cached := c.getFromCacheByKey(cacheKey); cached != "" {
		return cached, nil
	}

	var result string
	var err error

	switch mimeType {
	case "application/pdf":
		result, err = c.processPDFDirectly(ctx, fileBytes, prompt)
	case "image/jpeg", "image/jpg", "image/png", "image/webp", "image/tiff":
		result, err = c.processImageDirectly(ctx, fileBytes, prompt)
	case "text/plain":
		result, err = c.processTextDocument(ctx, fileBytes, prompt)
	default:
		return "", fmt.Errorf("unsupported file type: %s", mimeType)
	}

	if err != nil {
		return "", err
	}

	// Cache the result
	c.cacheResponseByKey(cacheKey, result)
	return result, nil
}

// Process PDF directly using document-capable models
func (c *CloudflareWorkerAIService) processPDFDirectly(ctx context.Context, pdfBytes []byte, prompt string) (string, error) {
	// List of models that can handle PDF documents directly
	models := []string{
		"@cf/meta/llama-3.2-90b-vision-instruct", // Best for complex documents
		"@cf/meta/llama-3.2-11b-vision-instruct", // Good for most documents
		"@cf/qwen/qwen2-vl-7b-instruct",          // Alternative vision model
	}

	// Convert PDF to base64
	pdfBase64 := base64.StdEncoding.EncodeToString(pdfBytes)

	fmt.Printf("Processing PDF of size: %d bytes (base64: %d chars)\n", len(pdfBytes), len(pdfBase64))

	for _, model := range models {
		fmt.Printf("Trying model: %s for PDF processing\n", model)

		result, err := c.callDocumentModel(ctx, model, pdfBase64, prompt, "application/pdf")
		if err != nil {
			fmt.Printf("Model %s failed: %v\n", model, err)
			time.Sleep(3 * time.Second) // Wait before trying next model
			continue
		}

		fmt.Printf("Model %s succeeded for PDF\n", model)
		return result, nil
	}

	// If all PDF models fail, try text extraction approach
	fmt.Println("All PDF models failed, trying text extraction approach...")
	return c.processPDFAsText(ctx, pdfBytes, prompt)
}

// Process images directly
func (c *CloudflareWorkerAIService) processImageDirectly(ctx context.Context, imageBytes []byte, prompt string) (string, error) {
	models := []string{
		"@cf/meta/llama-3.2-11b-vision-instruct",
		"@cf/meta/llava-hf/llava-1.5-7b-hf",
		"@cf/unum/uform-gen2-qwen-500m",
	}

	imageBase64 := base64.StdEncoding.EncodeToString(imageBytes)

	var lastErr error

	for _, model := range models {
		fmt.Printf("Trying model: %s for image processing\n", model)

		result, err := c.callDocumentModel(ctx, model, imageBase64, prompt, "image")
		if err != nil {
			fmt.Printf("Model %s failed: %v\n", model, err)
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}

		fmt.Printf("Model %s succeeded for image\n", model)
		return result, nil
	}

	return "", fmt.Errorf("all image models failed, last error: %w", lastErr)
}

// Fallback: Process PDF as extracted text
func (c *CloudflareWorkerAIService) processPDFAsText(ctx context.Context, pdfBytes []byte, prompt string) (string, error) {
	// This would require a PDF text extraction library
	// For now, return an informative error
	return "", fmt.Errorf("PDF direct processing failed and text extraction fallback not implemented")
}

// Process plain text documents
func (c *CloudflareWorkerAIService) processTextDocument(ctx context.Context, textBytes []byte, prompt string) (string, error) {
	combinedPrompt := fmt.Sprintf("%s\n\nDocument content:\n%s", prompt, string(textBytes))

	// Use text-based model for plain text
	return c.callTextModel(ctx, "@cf/meta/llama-3.1-70b-instruct", combinedPrompt)
}

// Call document-capable models (vision/multimodal)
func (c *CloudflareWorkerAIService) callDocumentModel(ctx context.Context, model, documentBase64, prompt, docType string) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s", c.accountID, model)

	var requestBody interface{}

	// Different request formats for different model types
	if strings.Contains(model, "vision") || strings.Contains(model, "llava") || strings.Contains(model, "qwen") {
		// Vision models expect image format even for PDFs
		requestBody = map[string]interface{}{
			"prompt": prompt,
			"image":  []string{documentBase64},
		}
	} else {
		// Document-specific models
		requestBody = CloudflareDocumentRequest{
			Prompt:   prompt,
			Document: documentBase64,
		}
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response CloudflareDocumentResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !response.Success {
		if len(response.Errors) > 0 {
			return "", fmt.Errorf("API error (code %d): %s", response.Errors[0].Code, response.Errors[0].Message)
		}
		return "", fmt.Errorf("API request was not successful")
	}

	// Try different response fields based on model type
	if response.Result.Response != "" {
		return response.Result.Response, nil
	}
	if response.Result.Description != "" {
		return response.Result.Description, nil
	}
	if response.Result.Text != "" {
		return response.Result.Text, nil
	}

	return "", fmt.Errorf("no valid response content found")
}

// Call text-based models
func (c *CloudflareWorkerAIService) callTextModel(ctx context.Context, model, prompt string) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s", c.accountID, model)

	request := CloudflareTextRequest{
		Messages: []CloudflareMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens: 4096,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response CloudflareDocumentResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !response.Success {
		if len(response.Errors) > 0 {
			return "", fmt.Errorf("API error (code %d): %s", response.Errors[0].Code, response.Errors[0].Message)
		}
		return "", fmt.Errorf("API request was not successful")
	}

	return response.Result.Response, nil
}

// Cache management methods
func (c *CloudflareWorkerAIService) getFromCacheByKey(key string) string {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()

	if cached, exists := c.cache[key]; exists {
		if time.Now().Before(cached.ExpiresAt) {
			return cached.Data
		}
		delete(c.cache, key)
	}
	return ""
}

func (c *CloudflareWorkerAIService) cacheResponseByKey(key, response string) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	c.cache[key] = &CachedResponse{
		Data:      response,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
}

func (c *CloudflareWorkerAIService) generateDocumentCacheKey(fileBytes []byte, prompt string) string {
	fileHash := md5.Sum(fileBytes)
	promptHash := md5.Sum([]byte(prompt))
	combined := append(fileHash[:], promptHash[:]...)
	finalHash := md5.Sum(combined)
	return hex.EncodeToString(finalHash[:])
}

func (c *CloudflareWorkerAIService) StartCacheCleanup() {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			c.cleanupExpiredCache()
		}
	}()
}

func (c *CloudflareWorkerAIService) cleanupExpiredCache() {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	now := time.Now()
	for key, cached := range c.cache {
		if now.After(cached.ExpiresAt) {
			delete(c.cache, key)
		}
	}
}
