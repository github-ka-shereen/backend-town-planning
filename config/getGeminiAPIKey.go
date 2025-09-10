package config

func GetGeminiAPIKey() string {
	key := GetEnv("GEMINI_API_KEY")
	if key == "" {
		Logger.Fatal("GEMINI_API_KEY is required in .env")
	}
	return key
}
