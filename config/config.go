package config

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	JWTSecret     string
	JWTTTL        time.Duration
	RedisAddr     string
	PublicBaseURL string // 网关对外可达地址（B9 接入配置生成用），空则用请求 Host 推导
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

	// Redis 限流（B7）
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// 对外可达地址（B9）：供接入配置生成，部署在反代/容器后建议显式设置
	publicBaseURL := strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/")

	return Config{Port: port, JWTSecret: jwtSecret, JWTTTL: jwtTTL, RedisAddr: redisAddr, PublicBaseURL: publicBaseURL}
}
