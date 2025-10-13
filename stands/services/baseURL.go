package services

import "os"

func GenerateDownloadLink(filePath string) string {
    port := os.Getenv("PORT")
    appEnv := os.Getenv("APP_ENV")

    baseURL := "http://localhost:" + port
    if appEnv == "production" {
        baseURL = os.Getenv("BASE_URL") // Make sure to set BASE_URL in production .env
    }

    return baseURL + filePath
}
