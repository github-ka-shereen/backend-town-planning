package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"town-planning-backend/config"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Enhanced DeviceFingerprint with better stability
type DeviceFingerprint struct {
	UserAgent     string `json:"user_agent"`
	ScreenRes     string `json:"screen_resolution"`
	Timezone      string `json:"timezone"`
	Language      string `json:"language"`
	Platform      string `json:"platform"`
	CookieEnabled bool   `json:"cookie_enabled"`
	IPAddress     string `json:"ip_address"` // Keep for logging, not for device ID
	Plugins       string `json:"plugins"`
	Canvas        string `json:"canvas_fingerprint"`
	WebGL         string `json:"webgl_fingerprint"`

	// Additional stable identifiers - KEEP AS STRING FOR FRACTIONAL VALUES
	ColorDepth          int     `json:"color_depth"`
	HardwareConcurrency int     `json:"hardware_concurrency"` // CPU cores
	DeviceMemory        float64 `json:"device_memory"`        // Available RAM (kept as string to handle fractional GB like "0.5")
	MaxTouchPoints      int     `json:"max_touch_points"`
}

type TrustedDevice struct {
	UserID       string            `json:"user_id"`
	DeviceID     string            `json:"device_id" gorm:"primaryKey"`
	DeviceName   string            `json:"device_name"`
	Fingerprint  DeviceFingerprint `json:"fingerprint" gorm:"type:jsonb"`
	RegisteredAt time.Time         `json:"registered_at"`
	LastUsedAt   time.Time         `json:"last_used_at"`
	IsActive     bool              `json:"is_active"`
}

type MagicLink struct {
	Token           string    `json:"token" gorm:"primaryKey"`
	URL             string    `json:"url"`
	VerificationURL string    `json:"verification_url"`
	ExpiresAt       time.Time `json:"expires_at"`
}

type MagicLinkData struct {
	Token             string            `json:"token" gorm:"primaryKey"`
	UserID            string            `json:"user_id"`
	Email             string            `json:"email"`
	DeviceFingerprint DeviceFingerprint `json:"device_fingerprint" gorm:"type:jsonb"`
	CreatedAt         time.Time         `json:"created_at"`
	ExpiresAt         time.Time         `json:"expires_at"`
	Used              bool              `json:"used"`
}

type MagicLinkPreferences struct {
	UserID                  string `json:"user_id" gorm:"primaryKey"`
	Enabled                 bool   `json:"enabled"`
	RequireDeviceVerify     bool   `json:"require_device_verify"`
	TrustNewDevices         bool   `json:"trust_new_devices"`
	LinkExpirationMinutes   int    `json:"link_expiration_minutes"`
	MaxActiveLinks          int    `json:"max_active_links"`
	RequireRecentAuthForUse bool   `json:"require_recent_auth_for_use"`
}

type DeviceService struct {
	redisClient     *redis.Client
	ctx             context.Context
	frontendBaseURL string
	deviceTTL       time.Duration // Configurable TTL for device records
}

type SecurityEvent struct {
	UserID    string         `json:"user_id"`
	EventType string         `json:"event_type"` // "suspicious_login", "magic_link_used", "device_change"
	IPAddress string         `json:"ip_address"`
	UserAgent string         `json:"user_agent"`
	Timestamp time.Time      `json:"timestamp"`
	Details   map[string]any `json:"details"`
}

func NewDeviceService(redisClient *redis.Client, ctx context.Context, frontendBaseURL string, deviceTTL time.Duration) *DeviceService {
	return &DeviceService{
		redisClient:     redisClient,
		ctx:             ctx,
		frontendBaseURL: frontendBaseURL,
		deviceTTL:       deviceTTL, // Set to 0 if you don't want expiration
	}
}

func GenerateDeviceID(deviceFingerprint DeviceFingerprint) string {
	// Use only the most stable characteristics that rarely change
	// Exclude: IP Address, Plugins (can change frequently)
	// Include: Hardware-based and browser-intrinsic characteristics

	// Normalize user agent to extract stable browser info
	normalizedUA := normalizeUserAgent(deviceFingerprint.UserAgent)

	fingerprintString := fmt.Sprintf("%s|%s|%s|%s|%s|%v|%s|%s|%d|%d|%f|%d",
		normalizedUA,                          // Stable browser signature
		deviceFingerprint.ScreenRes,           // Screen resolution (very stable)
		deviceFingerprint.Platform,            // OS platform (stable)
		deviceFingerprint.Canvas,              // Hardware-based rendering (very stable)
		deviceFingerprint.WebGL,               // GPU-based rendering (very stable)
		deviceFingerprint.CookieEnabled,       // Browser setting (stable)
		deviceFingerprint.Timezone,            // Geographic/system setting (relatively stable)
		deviceFingerprint.Language,            // System language (stable)
		deviceFingerprint.ColorDepth,          // Display capability (very stable)
		deviceFingerprint.HardwareConcurrency, // CPU cores (hardware-based, very stable)
		deviceFingerprint.DeviceMemory,        // RAM (hardware-based, very stable)
		deviceFingerprint.MaxTouchPoints,      // Touch capability (hardware-based, stable)
	)

	hash := sha256.Sum256([]byte(fingerprintString))
	return "v1:" + hex.EncodeToString(hash[:])
}

// normalizeUserAgent extracts stable browser information while removing version-specific details
func normalizeUserAgent(userAgent string) string {
	ua := strings.ToLower(userAgent)

	// Extract major browser family and OS, ignore specific versions
	var browserFamily, osFamily string

	// Browser detection
	if strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg") {
		browserFamily = "chrome"
	} else if strings.Contains(ua, "firefox") {
		browserFamily = "firefox"
	} else if strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome") {
		browserFamily = "safari"
	} else if strings.Contains(ua, "edg") {
		browserFamily = "edge"
	} else {
		browserFamily = "other"
	}

	// OS detection
	if strings.Contains(ua, "Windows") {
		osFamily = "windows"
	} else if strings.Contains(ua, "macOS") {
		osFamily = "macos"
	} else if strings.Contains(ua, "Linux") {
		osFamily = "linux"
	} else if strings.Contains(ua, "Android") {
		osFamily = "android"
	} else if strings.Contains(ua, "iPhone") || strings.Contains(ua, "iphone") || strings.Contains(ua, "iPad") || strings.Contains(ua, "ipad") {
		osFamily = "ios"
	} else {
		osFamily = "other"
	}

	return fmt.Sprintf("%s_%s", browserFamily, osFamily)
}

// Updated RegisterDevice with TTL support
func (ds *DeviceService) RegisterDevice(userID string, deviceFingerprint DeviceFingerprint) (*TrustedDevice, error) {
	deviceID := GenerateDeviceID(deviceFingerprint)

	device := TrustedDevice{
		UserID:       userID,
		DeviceID:     deviceID,
		DeviceName:   ds.generateDeviceName(deviceFingerprint),
		Fingerprint:  deviceFingerprint,
		RegisteredAt: time.Now(),
		LastUsedAt:   time.Now(),
		IsActive:     true,
	}

	jsonData, err := json.Marshal(device)
	if err != nil {
		return nil, err
	}

	redisKey := fmt.Sprintf("trusted_device:%s:%s", userID, deviceID)

	// Set with TTL if configured
	if ds.deviceTTL > 0 {
		err = ds.redisClient.SetEx(ds.ctx, redisKey, string(jsonData), ds.deviceTTL).Err()
	} else {
		err = ds.redisClient.Set(ds.ctx, redisKey, string(jsonData), 0).Err()
	}

	if err != nil {
		return nil, err
	}

	return &device, nil
}

// Check if user is under security lockdown
func (ds *DeviceService) IsUserLocked(userID string) (bool, error) {
	lockdownKey := fmt.Sprintf("security_lockdown:%s", userID)
	_, err := ds.redisClient.Get(ds.ctx, lockdownKey).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (ds *DeviceService) IsDeviceTrusted(userID string, deviceFingerprint DeviceFingerprint) (bool, *TrustedDevice, error) {
	// Check if user is locked
	isLocked, err := ds.IsUserLocked(userID)
	if err != nil {
		return false, nil, fmt.Errorf("failed to check lockdown status: %w", err)
	}
	if isLocked {
		return false, nil, fmt.Errorf("account is locked for security reasons")
	}

	deviceID := GenerateDeviceID(deviceFingerprint)
	redisKey := fmt.Sprintf("trusted_device:%s:%s", userID, deviceID)

	data, err := ds.redisClient.Get(ds.ctx, redisKey).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil, nil // Device not found (not an error)
		}
		return false, nil, fmt.Errorf("redis error: %w", err)
	}

	var device TrustedDevice
	if err := json.Unmarshal([]byte(data), &device); err != nil {
		return false, nil, fmt.Errorf("unmarshal error: %w", err)
	}

	// Update LastUsedAt
	device.LastUsedAt = time.Now()
	updatedData, err := json.Marshal(device)
	if err != nil {
		return false, nil, fmt.Errorf("marshal error: %w", err)
	}

	// Set with TTL refresh if configured
	if ds.deviceTTL > 0 {
		err = ds.redisClient.SetEx(ds.ctx, redisKey, string(updatedData), ds.deviceTTL).Err()
	} else {
		err = ds.redisClient.Set(ds.ctx, redisKey, string(updatedData), 0).Err()
	}

	if err != nil {
		return false, nil, fmt.Errorf("redis set error: %w", err)
	}

	return device.IsActive, &device, nil
}

// Updated GetTrustedDevices using SCAN instead of KEYS
func (ds *DeviceService) GetTrustedDevices(userID string) ([]TrustedDevice, error) {
	var devices []TrustedDevice
	var cursor uint64
	var err error
	pattern := fmt.Sprintf("trusted_device:%s:*", userID)

	// Use SCAN to safely iterate through keys in production
	for {
		var keys []string
		keys, cursor, err = ds.redisClient.Scan(ds.ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan device keys: %w", err)
		}

		for _, key := range keys {
			data, err := ds.redisClient.Get(ds.ctx, key).Result()
			if err != nil || data == "" { // Skip missing/corrupted keys
				continue
			}

			var device TrustedDevice
			if err := json.Unmarshal([]byte(data), &device); err == nil {
				devices = append(devices, device)
			}
		}

		if cursor == 0 {
			break
		}
	}

	return devices, nil
}

// Updated UpdateDevice with TTL preservation
func (ds *DeviceService) UpdateDevice(userID string, device TrustedDevice) error {
	jsonData, err := json.Marshal(device)
	if err != nil {
		return fmt.Errorf("failed to marshal device: %w", err)
	}

	redisKey := fmt.Sprintf("trusted_device:%s:%s", userID, device.DeviceID)

	// If no TTL configured, persist indefinitely
	if ds.deviceTTL <= 0 {
		return ds.redisClient.Set(ds.ctx, redisKey, jsonData, 0).Err()
	}

	// Handle TTL cases
	ttl, err := ds.redisClient.TTL(ds.ctx, redisKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get TTL for key %s: %w", redisKey, err)
	}

	switch {
	case ttl == -2: // Key doesn't exist or expired
		return ds.redisClient.SetEx(ds.ctx, redisKey, jsonData, ds.deviceTTL).Err()
	case ttl == -1: // Key exists with no TTL
		return ds.redisClient.SetEx(ds.ctx, redisKey, jsonData, ds.deviceTTL).Err()
	default: // Key exists with TTL - preserve existing TTL
		return ds.redisClient.SetEx(ds.ctx, redisKey, jsonData, ttl).Err()
	}
}

func (ds *DeviceService) RemoveTrustedDevice(userID, deviceID string) error {
	redisKey := fmt.Sprintf("trusted_device:%s:%s", userID, deviceID)
	return ds.redisClient.Del(ds.ctx, redisKey).Err()
}

// Remove all trusted devices for a user
func (ds *DeviceService) RemoveAllTrustedDevices(userID string) error {
	pattern := fmt.Sprintf("trusted_device:%s:*", userID)
	var cursor uint64

	for {
		keys, newCursor, err := ds.redisClient.Scan(ds.ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan trusted devices: %w", err)
		}

		for _, key := range keys {
			ds.redisClient.Del(ds.ctx, key)
		}

		cursor = newCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

func (ds *DeviceService) LockdownUser(userID string, reason string) error {
	// Log security event
	event := SecurityEvent{
		UserID:    userID,
		EventType: "account_lockdown",
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"reason": reason,
		},
	}
	ds.logSecurityEvent(event)

	// // Revoke all sessions
	// if err := ds.RevokeAllUserSessions(userID); err != nil {
	// 	return fmt.Errorf("failed to revoke sessions: %w", err)
	// }

	// // Invalidate all magic links
	// if err := ds.InvalidateAllUserMagicLinks(userID); err != nil {
	// 	return fmt.Errorf("failed to invalidate magic links: %w", err)
	// }

	// Remove all trusted devices
	if err := ds.RemoveAllTrustedDevices(userID); err != nil {
		return fmt.Errorf("failed to remove trusted devices: %w", err)
	}

	// Set security flag
	lockdownKey := fmt.Sprintf("security_lockdown:%s", userID)
	lockdownData := map[string]interface{}{
		"locked_at": time.Now(),
		"reason":    reason,
		"status":    "locked",
	}

	jsonData, _ := json.Marshal(lockdownData)
	return ds.redisClient.Set(ds.ctx, lockdownKey, string(jsonData), 0).Err()
}

// Unlock user (admin action)
func (ds *DeviceService) UnlockUser(userID string, adminID string) error {
	lockdownKey := fmt.Sprintf("security_lockdown:%s", userID)

	// Log unlock event
	event := SecurityEvent{
		UserID:    userID,
		EventType: "account_unlocked",
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"admin_id": adminID,
		},
	}
	ds.logSecurityEvent(event)

	return ds.redisClient.Del(ds.ctx, lockdownKey).Err()
}

func (ds *DeviceService) logSecurityEvent(event SecurityEvent) {
	eventKey := fmt.Sprintf("security_event:%s:%d", event.UserID, time.Now().Unix())
	jsonData, _ := json.Marshal(event)

	// Store for 30 days
	ds.redisClient.SetEx(ds.ctx, eventKey, string(jsonData), 30*24*time.Hour)

	// Also log to application logger
	config.Logger.Warn("Security event",
		zap.String("user_id", event.UserID),
		zap.String("event_type", event.EventType),
		zap.String("ip_address", event.IPAddress),
		zap.Any("details", event.Details),
	)
}

func (ds *DeviceService) generateDeviceName(df DeviceFingerprint) string {
	browserName, _ := extractBrowserInfo(df.UserAgent)
	return fmt.Sprintf("%s/%s Registered on %s",
		df.Platform,
		browserName,
		time.Now().Format("Jan 2, 2006"))
}

// extractBrowserInfo parses user agent to get readable browser name and version
func extractBrowserInfo(userAgent string) (string, string) {
	ua := strings.ToLower(userAgent)

	// Chrome (must be before Safari check)
	if strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg") {
		re := regexp.MustCompile(`chrome/(\d+)`)
		if matches := re.FindStringSubmatch(ua); len(matches) > 1 {
			return "Chrome", matches[1]
		}
		return "Chrome", "Unknown"
	}

	// Edge
	if strings.Contains(ua, "edg") {
		re := regexp.MustCompile(`edg/(\d+)`)
		if matches := re.FindStringSubmatch(ua); len(matches) > 1 {
			return "Edge", matches[1]
		}
		return "Edge", "Unknown"
	}

	// Firefox
	if strings.Contains(ua, "firefox") {
		re := regexp.MustCompile(`firefox/(\d+)`)
		if matches := re.FindStringSubmatch(ua); len(matches) > 1 {
			return "Firefox", matches[1]
		}
		return "Firefox", "Unknown"
	}

	// Safari (must be after Chrome check)
	if strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome") {
		re := regexp.MustCompile(`version/(\d+)`)
		if matches := re.FindStringSubmatch(ua); len(matches) > 1 {
			return "Safari", matches[1]
		}
		return "Safari", "Unknown"
	}

	// Opera
	if strings.Contains(ua, "opera") || strings.Contains(ua, "opr") {
		re := regexp.MustCompile(`(?:opera|opr)/(\d+)`)
		if matches := re.FindStringSubmatch(ua); len(matches) > 1 {
			return "Opera", matches[1]
		}
		return "Opera", "Unknown"
	}

	return "Unknown Browser", ""
}
