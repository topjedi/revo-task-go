package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
)

func init() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found, will be used os env")
	}
}

func GetEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
