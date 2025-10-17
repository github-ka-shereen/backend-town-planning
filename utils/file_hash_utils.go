package utils

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"regexp"
	"strings"
)

// GenerateFileHash generates MD5 hash of a file
func GenerateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	hashInBytes := hash.Sum(nil)[:16]
	return hex.EncodeToString(hashInBytes), nil
}

// CleanStringForFilename cleans a string for safe use in filenames
func CleanStringForFilename(input string) string {
	// Remove or replace problematic characters
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == ' ' || r == '-':
			return '_'
		case r == '.':
			return '.'
		default:
			return -1 // remove
		}
	}, input)

	// Remove multiple consecutive underscores
	clean = regexp.MustCompile(`_+`).ReplaceAllString(clean, "_")
	// Trim underscores from start and end
	clean = strings.Trim(clean, "_")

	if clean == "" {
		clean = "file"
	}

	// Limit length
	if len(clean) > 100 {
		clean = clean[:100]
	}

	return clean
}
