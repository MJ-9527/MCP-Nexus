package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	JWTSecret      string
	JWTTTL         time.Duration
	RedisAddr      string
	ClickHouseAddr string
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("未加载.env,将使用系统环境变量")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "mcp-nexus-dev-secret-change-in-prod"
	}

	jwtTTL := 24 * time.Hour
	if v := os.Getenv("JWT_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			jwtTTL = d
		}
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	clickHouseAddr := os.Getenv("CLICKHOUSE_ADDR")
	if clickHouseAddr == "" {
		clickHouseAddr = "localhost:9000"
	}

	return Config{
		Port:           port,
		JWTSecret:      jwtSecret,
		JWTTTL:         jwtTTL,
		RedisAddr:      redisAddr,
		ClickHouseAddr: clickHouseAddr,
	}
}
