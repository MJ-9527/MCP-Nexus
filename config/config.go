package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port string
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("未加载.env,将使用系统环境变量")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return Config{port}
}
