package config

import (
	"fmt"
	"os"
)

type Config struct {
	WeatherAPIKey string
	ServerPort    string
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		WeatherAPIKey: os.Getenv("WEATHER_API_KEY"),
		ServerPort:    os.Getenv("SERVER_PORT"),
	}

	if cfg.WeatherAPIKey == "" {
		return nil, fmt.Errorf("WEATHER_API_KEY is not set; please provide it via environment variable")
	}

	if cfg.ServerPort == "" {
		cfg.ServerPort = ":8081"
	} else if cfg.ServerPort[0] != ':' {
		cfg.ServerPort = ":" + cfg.ServerPort
	}

	return cfg, nil
}
