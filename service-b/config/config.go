package config

import (
	"context"
	"fmt"
	"os"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/spf13/viper"
)

// Config holds the configuration parameters for the rate limiter service.
// It includes settings for request limits, lockout durations, window size, and Redis connection details.
// Fields:
//   - WeatherAPIToken: API key to the weather service.
//   - ServerPort: Port on which the server listens for incoming requests.
type Config struct {
	WeatherAPIKey string `mapstructure:"WEATHER_API_KEY"`
	ServerPort    string `mapstructure:"SERVER_PORT"`
}

// LoadConfig reads config from environment, optional .env file, and Secret Manager
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Read from environment variables
	v.AutomaticEnv()
	v.BindEnv("SERVER_PORT")
	v.BindEnv("WEATHER_API_KEY")

	// Check for an optional .env file for local development.
	// This will be overridden by environment variables if they are set.
	envFilePath := ".env"
	if _, err := os.Stat(envFilePath); err == nil {
		v.SetConfigFile(envFilePath)
		v.SetConfigType("env")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("could not read .env file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// If WEATHER_API_KEY is empty, fetch it from Secret Manager
	if cfg.WeatherAPIKey == "" {
		secretKey, err := getWeatherAPIKey()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch WEATHER_API_KEY from Secret Manager: %w", err)
		}
		cfg.WeatherAPIKey = secretKey
	}

	// Ensure ServerPort is set (default to ":8080")
	if cfg.ServerPort == "" {
		cfg.ServerPort = ":8080"
	} else if cfg.ServerPort[0] != ':' {
		cfg.ServerPort = ":" + cfg.ServerPort
	}

	return &cfg, nil
}

// getWeatherAPIKey fetches the secret from Secret Manager
func getWeatherAPIKey() (string, error) {
	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create secretmanager client: %w", err)
	}
	defer client.Close()

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		return "", fmt.Errorf("GOOGLE_CLOUD_PROJECT environment variable not set")
	}

	secretName := fmt.Sprintf("projects/%s/secrets/WEATHER_API_KEY/versions/latest", projectID)
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName,
	}

	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret version: %w", err)
	}

	return string(result.Payload.Data), nil
}
