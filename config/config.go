package config

import "os"

type Config struct {
	Path     string
	Port     string
	Username string
	Password string
	Domain   string
}

func Load() *Config {
	cfg := &Config{
		Path:     getEnv("DATA_PATH", "./data"),
		Port:     getEnv("PORT", "5000"),
		Username: getEnv("SERVER_USERNAME", "user"),
		Password: getEnv("SERVER_PASSWORD", "test123"),
		Domain:   getEnv("IMAGE_SERVER_DOMAIN", "https://image.mindustry-tool.app"),
	}
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}