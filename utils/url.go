package utils

import (
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// GetDownloadURL generates a download URL based on the environment (http for development, https for production).
func GetDownloadURL(c *fiber.Ctx, filePath string) string {
	// Check if the app is in production or development
	env := os.Getenv("APP_ENV") // Assumes you have set this in your environment
	// Remove leading slash if it exists
	filePath = strings.TrimPrefix(filePath, "/")

	if env == "production" {
		return fmt.Sprintf("https://%s/%s", c.Hostname(), filePath)
	}
	// Default to "http" for development
	return fmt.Sprintf("http://%s/%s", c.Hostname(), filePath)
}
