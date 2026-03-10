package config

import "os"

type Config struct {
	DBUser        string
	DBPassword    string
	DBHost        string
	DBPort        string
	DBName        string
	RedisAddr     string
	RedisPassword string
	OpenAIKey     string
	GeminiKey     string
}

func Load() Config {
	return Config{
		DBUser:        os.Getenv("DB_USER"),
		DBPassword:    os.Getenv("DB_PASSWORD"),
		DBHost:        os.Getenv("DB_HOST"),
		DBPort:        os.Getenv("DB_PORT"),
		DBName:        os.Getenv("DB_NAME"),
		RedisAddr:     os.Getenv("REDIS_ADDR"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		OpenAIKey:     os.Getenv("OPENAI_API_KEY"),
		GeminiKey:     os.Getenv("GEMINI_API_KEY"),
	}
}
