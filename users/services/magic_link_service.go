package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"town-planning-backend/config"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type MagicLinkService struct {
	redisClient     *redis.Client
	ctx             context.Context
	baseURL         string
	frontendBaseURL string
}

func NewMagicLinkService(redisClient *redis.Client, ctx context.Context, baseURL, frontendBaseURL string) *MagicLinkService {
	return &MagicLinkService{
		redisClient:     redisClient,
		ctx:             ctx,
		baseURL:         baseURL,
		frontendBaseURL: frontendBaseURL,
	}
}

func (mls *MagicLinkService) GenerateMagicLink(userID, email string, deviceFingerprint DeviceFingerprint) (*MagicLink, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		config.Logger.Error("Failed to generate magic link token", zap.Error(err))
		return nil, err
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	expiresAt := time.Now().Add(15 * time.Minute)
	magicLinkData := MagicLinkData{
		UserID:            userID,
		Email:             email,
		DeviceFingerprint: deviceFingerprint,
		CreatedAt:         time.Now(),
		ExpiresAt:         expiresAt,
		Used:              false,
	}

	jsonData, err := json.Marshal(magicLinkData)
	if err != nil {
		return nil, err
	}

	redisKey := "magic_link:" + token
	if err := mls.redisClient.Set(mls.ctx, redisKey, string(jsonData), 15*time.Minute).Err(); err != nil {
		return nil, err
	}

	// Generate the verification URL that will hit your backend
	verificationURL := fmt.Sprintf("%s/api/v1/auth/magiclink/verify?token=%s", mls.baseURL, token)

	// Generate the frontend redirect URL that will be sent to the user
	magicURL := fmt.Sprintf("%s/auth/magic-login?token=%s", mls.frontendBaseURL, token)

	return &MagicLink{
		Token:           token,
		URL:             magicURL,
		VerificationURL: verificationURL,
		ExpiresAt:       expiresAt,
	}, nil
}

func (mls *MagicLinkService) ValidateMagicLink(token string, deviceFingerprint DeviceFingerprint) (*MagicLinkData, string, error) {

	redisKey := "magic_link:" + token
	data, err := mls.redisClient.Get(mls.ctx, redisKey).Result()
	if err != nil {
		return nil, "", err
	}

	var magicLinkData MagicLinkData
	if err := json.Unmarshal([]byte(data), &magicLinkData); err != nil {
		return nil, "", err
	}

	if magicLinkData.Used {
		return nil, "", fmt.Errorf("magic link already used")
	}

	if time.Now().After(magicLinkData.ExpiresAt) {
		mls.InvalidateMagicLink(token)
		return nil, "", fmt.Errorf("magic link expired")
	}

	if !mls.isDeviceFingerprintSimilar(magicLinkData.DeviceFingerprint, deviceFingerprint) {
		return nil, "", fmt.Errorf("device fingerprint mismatch")
	}

	// Generate redirect URL with token for frontend
	redirectURL := fmt.Sprintf("%s/auth/magic-callback?token=%s", mls.frontendBaseURL, token)

	magicLinkData.Used = true
	updatedData, _ := json.Marshal(magicLinkData)
	mls.redisClient.Set(mls.ctx, redisKey, string(updatedData), time.Until(magicLinkData.ExpiresAt))

	return &magicLinkData, redirectURL, nil
}

func (mls *MagicLinkService) InvalidateMagicLink(token string) {
	mls.redisClient.Del(mls.ctx, "magic_link:"+token)
}

func (mls *MagicLinkService) isDeviceFingerprintSimilar(stored, current DeviceFingerprint) bool {
	score := 0
	total := 10

	if stored.UserAgent == current.UserAgent {
		score++
	}
	if stored.ScreenRes == current.ScreenRes {
		score++
	}
	if stored.Timezone == current.Timezone {
		score++
	}
	if stored.Language == current.Language {
		score++
	}
	if stored.Platform == current.Platform {
		score++
	}
	if stored.CookieEnabled == current.CookieEnabled {
		score++
	}
	if stored.Plugins == current.Plugins {
		score++
	}
	if stored.Canvas == current.Canvas {
		score++
	}
	if stored.WebGL == current.WebGL {
		score++
	}

	return float64(score)/float64(total) >= 0.7
}
