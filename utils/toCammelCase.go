package utils

import "strings"

func ToCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(string(part[0])) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, "")
}
