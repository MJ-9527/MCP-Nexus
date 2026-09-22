package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"MCP-Nexus/pkg/logutil"
	"MCP-Nexus/pkg/security"
	"MCP-Nexus/router"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := logutil.SetupLogger(os.Stderr)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	// 可选：覆盖敏感工具的 API Key。
	if k := os.Getenv("API_KEY"); k != "" {
		security.ServiceAPIKey = k
	}

	// 可选：配置 DATABASE_URL 后启用数据库类工具。
	var db *pgxpool.Pool
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pool, err := pgxpool.New(ctx, dsn)
		cancel()
		if err != nil {
			logger.Error("connect database failed", slog.String("error", err.Error()))
			panic(err)
		}
		db = pool
		defer db.Close()
	}

	fileBase := os.Getenv("FILE_BASE_DIR")

	logger.Info("demo-service starting", slog.String("port", port))
	if err := router.SetupDemoRouter(db, fileBase).Run(":" + port); err != nil {
		logger.Error("server exited", slog.String("error", err.Error()))
		panic(err)
	}
}
