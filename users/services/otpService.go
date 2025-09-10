package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"town-planning-backend/config"

	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Enhanced interface with TOTP methods
type OtpService interface {
	// Email OTP methods (existing)
	GenerateOtp(keySuffix string) (otp string, pre_token string)
	ValidateOtp(otp string, pre_token string, keySuffix string) bool
	InvalidateOtp(keySuffix string)

	// TOTP methods (new)
	GenerateTOTPSecret(userID, email string) (*TOTPSetup, error)
	ValidateTOTPCode(userID, code string) bool
	EnableTOTP(userID, code string) error
	DisableTOTP(userID string) error
	IsTOTPEnabled(userID string) bool
}

type TOTPSetup struct {
	Secret    string `json:"secret"`
	QRCodeURL string `json:"qr_code_url"`
	ManualKey string `json:"manual_key"`
}

type TOTPData struct {
	Secret    string    `json:"secret"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

type otpService struct {
	redisClient *redis.Client
	ctx         context.Context
}

func NewOtpService(redisClient *redis.Client, ctx context.Context) OtpService {
	return &otpService{redisClient: redisClient, ctx: ctx}
}

type storagePayload struct {
	PreToken string `json:"pre_token"`
	Otp      string `json:"otp"`
}

// Existing email OTP methods remain the same
func (os *otpService) GenerateOtp(keySuffix string) (otp string, pre_token string) {
	otpValue, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		config.Logger.Error("Failed to generate random OTP", zap.Error(err))
		return "", ""
	}
	otp = fmt.Sprintf("%06d", otpValue.Int64()+100000)

	preTokenBytes := make([]byte, 16)
	_, err = rand.Read(preTokenBytes)
	if err != nil {
		config.Logger.Error("Failed to generate random pre-token", zap.Error(err))
		return "", ""
	}
	pre_token = base64.URLEncoding.EncodeToString(preTokenBytes)

	payload := storagePayload{
		PreToken: pre_token,
		Otp:      otp,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		config.Logger.Error("Failed to marshal OTP payload", zap.Error(err))
		return "", ""
	}

	redisKey := "otp:" + keySuffix
	const otpDuration = 5 * time.Minute
	err = os.redisClient.Set(os.ctx, redisKey, string(jsonData), otpDuration).Err()
	if err != nil {
		config.Logger.Error("Failed to set OTP in Redis", zap.Error(err), zap.String("key", redisKey))
		return "", ""
	}

	return otp, pre_token
}

func (os *otpService) ValidateOtp(otp string, pre_token string, keySuffix string) bool {
	redisKey := "otp:" + keySuffix
	data := os.redisClient.Get(os.ctx, redisKey).Val()
	if data == "" {
		config.Logger.Warn("OTP not found or expired in Redis", zap.String("key", redisKey))
		return false
	}

	var storagePayload storagePayload
	err := json.Unmarshal([]byte(data), &storagePayload)
	if err != nil {
		config.Logger.Error("Failed to unmarshal OTP payload from Redis", zap.Error(err), zap.String("key", redisKey))
		return false
	}

	if storagePayload.PreToken == pre_token && storagePayload.Otp == otp {
		os.InvalidateOtp(keySuffix)
		return true
	}

	config.Logger.Warn("Invalid OTP or pre-token provided",
		zap.String("key", redisKey),
		zap.String("provided_otp", otp),
		zap.String("provided_pre_token_hint", pre_token[:5]+"..."),
		zap.String("stored_otp", storagePayload.Otp),
		zap.String("stored_pre_token_hint", storagePayload.PreToken[:5]+"..."),
	)
	return false
}

func (os *otpService) InvalidateOtp(keySuffix string) {
	redisKey := "otp:" + keySuffix
	err := os.redisClient.Del(os.ctx, redisKey).Err()
	if err != nil {
		config.Logger.Error("Failed to invalidate OTP in Redis", zap.Error(err), zap.String("key", redisKey))
	}
}

// New TOTP methods
func (os *otpService) GenerateTOTPSecret(userID, email string) (*TOTPSetup, error) {
	// Generate TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "AcrePoint", // Replace with your app name
		AccountName: email,
		SecretSize:  32,
	})
	if err != nil {
		config.Logger.Error("Failed to generate TOTP secret", zap.Error(err))
		return nil, err
	}

	// Store the secret in Redis (but don't enable it yet)
	totpData := TOTPData{
		Secret:    key.Secret(),
		Enabled:   false,
		CreatedAt: time.Now(),
	}

	jsonData, err := json.Marshal(totpData)
	if err != nil {
		config.Logger.Error("Failed to marshal TOTP data", zap.Error(err))
		return nil, err
	}

	redisKey := "totp:" + userID
	// Store for 10 minutes to allow setup
	err = os.redisClient.Set(os.ctx, redisKey, string(jsonData), 10*time.Minute).Err()
	if err != nil {
		config.Logger.Error("Failed to store TOTP secret in Redis", zap.Error(err))
		return nil, err
	}

	setup := &TOTPSetup{
		Secret:    key.Secret(),
		QRCodeURL: key.URL(),
		ManualKey: key.Secret(),
	}

	return setup, nil
}

func (os *otpService) ValidateTOTPCode(userID, code string) bool {
	redisKey := "totp:" + userID
	data := os.redisClient.Get(os.ctx, redisKey).Val()
	if data == "" {
		config.Logger.Warn("TOTP data not found for user", zap.String("userID", userID))
		return false
	}

	var totpData TOTPData
	err := json.Unmarshal([]byte(data), &totpData)
	if err != nil {
		config.Logger.Error("Failed to unmarshal TOTP data", zap.Error(err))
		return false
	}

	// Validate the TOTP code
	valid := totp.Validate(code, totpData.Secret)
	if !valid {
		config.Logger.Warn("Invalid TOTP code provided", zap.String("userID", userID))
	}

	return valid
}

func (os *otpService) EnableTOTP(userID, code string) error {
	// First validate the code
	if !os.ValidateTOTPCode(userID, code) {
		return fmt.Errorf("invalid TOTP code")
	}

	// Get the current TOTP data
	redisKey := "totp:" + userID
	data := os.redisClient.Get(os.ctx, redisKey).Val()
	if data == "" {
		return fmt.Errorf("TOTP setup not found")
	}

	var totpData TOTPData
	err := json.Unmarshal([]byte(data), &totpData)
	if err != nil {
		return err
	}

	// Enable TOTP
	totpData.Enabled = true

	jsonData, err := json.Marshal(totpData)
	if err != nil {
		return err
	}

	// Store permanently (no expiration)
	err = os.redisClient.Set(os.ctx, redisKey, string(jsonData), 0).Err()
	if err != nil {
		config.Logger.Error("Failed to enable TOTP in Redis", zap.Error(err))
		return err
	}

	config.Logger.Info("TOTP enabled for user", zap.String("userID", userID))
	return nil
}

func (os *otpService) DisableTOTP(userID string) error {
	redisKey := "totp:" + userID
	err := os.redisClient.Del(os.ctx, redisKey).Err()
	if err != nil {
		config.Logger.Error("Failed to disable TOTP in Redis", zap.Error(err))
		return err
	}

	config.Logger.Info("TOTP disabled for user", zap.String("userID", userID))
	return nil
}

func (os *otpService) IsTOTPEnabled(userID string) bool {
	redisKey := "totp:" + userID
	data := os.redisClient.Get(os.ctx, redisKey).Val()
	if data == "" {
		return false
	}

	var totpData TOTPData
	err := json.Unmarshal([]byte(data), &totpData)
	if err != nil {
		config.Logger.Error("Failed to unmarshal TOTP data", zap.Error(err))
		return false
	}

	return totpData.Enabled
}
