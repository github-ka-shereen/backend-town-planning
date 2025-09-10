package config

import (
	"context"
	"log"
	"os"

	"github.com/elastic/go-elasticsearch/v8"
)

// InitElasticsearch initializes the Elasticsearch client
func InitElasticsearch(ctx context.Context) *elasticsearch.Client {
	// Get the environment (defaults to "development" if not set)
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	var esAddress string
	if env == "production" {
		// In production, use HTTPS
		esAddress = GetEnv("ELASTICSEARCH_ADDRESS") // Ensure this points to your HTTPS endpoint
	} else {
		// In development, use HTTP
		esAddress = GetEnv("ELASTICSEARCH_ADDRESS") // Ensure this points to your HTTP endpoint
	}

	cfg := elasticsearch.Config{
		Addresses: []string{
			esAddress, // Use the address from your environment
		},
		Username: GetEnv("ELASTICSEARCH_USERNAME"),
        Password: GetEnv("ELASTICSEARCH_PASSWORD"),
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error initializing Elasticsearch: %s", err)
	}

	// Test the connection using the Info API
	res, err := client.Info()
	if err != nil {
		log.Fatalf("Error connecting to Elasticsearch: %s", err)
	}
	defer res.Body.Close()

	log.Println("Elasticsearch is up and running")
	return client
}
