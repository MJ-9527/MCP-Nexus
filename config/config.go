package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port      string
	JWTSecret string
	JWTTTL    time.Duration
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("未加载.env,将使用系统环境变量")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// JWT 配置（B5）：密钥缺失时使用开发默认值，生产环境必须显式设置
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "mcp-nexus-dev-secret-change-in-prod"
	}
	jwtTTL := 24 * time.Hour
	if raw := os.Getenv("JWT_TTL"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			jwtTTL = d
		}
	}

	return Config{Port: port, JWTSecret: jwtSecret, JWTTTL: jwtTTL}
}
