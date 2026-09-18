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

	// B14：ClickHouse 审计分析存储（走 HTTP 接口）。
	// Addr 为空表示不启用该分析存储：审计仍写 PostgreSQL，行为与之前完全一致。
	ClickHouseAddr string // 形如 clickhouse:8123 或 http://clickhouse:8123
	ClickHouseDB   string // 数据库名，默认 mcp_analytics
	ClickHouseUser string
	ClickHousePass string
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

	// ClickHouse 审计分析存储（B14）：未配置则审计只落 PostgreSQL
	chDB := os.Getenv("CLICKHOUSE_DB")
	if chDB == "" {
		chDB = "mcp_analytics"
	}

	return Config{
		Port:           port,
		JWTSecret:      jwtSecret,
		JWTTTL:         jwtTTL,
		RedisAddr:      redisAddr,
		PublicBaseURL:  publicBaseURL,
		ClickHouseAddr: strings.TrimRight(strings.TrimSpace(os.Getenv("CLICKHOUSE_HTTP_ADDR")), "/"),
		ClickHouseDB:   chDB,
		ClickHouseUser: os.Getenv("CLICKHOUSE_USER"),
		ClickHousePass: os.Getenv("CLICKHOUSE_PASSWORD"),
	}
}
