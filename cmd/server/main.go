package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"MCP-Nexus/config"
	"MCP-Nexus/router"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := config.NewPostgresPool(ctx)
	if err != nil {
		log.Fatal("连接 PostgreSQL 失败：", err)
	}
	defer pool.Close()

	var clickhouseConn clickhouse.Conn
	clickhouseConn, clickhouseErr := config.NewClickHouse(ctx, cfg)
	if clickhouseErr != nil {
		log.Printf("连接 ClickHouse 失败，分析审计将禁用但主服务继续运行：%v", clickhouseErr)
	} else {
		schema, readErr := os.ReadFile("db/clickhouse/001_a13_audit_logs.sql")
		if readErr != nil {
			log.Printf("读取 ClickHouse schema 失败：%v", readErr)
		} else if execErr := executeSchema(ctx, clickhouseConn, string(schema)); execErr != nil {
			log.Printf("初始化 ClickHouse schema 失败：%v", execErr)
		} else {
			log.Printf("ClickHouse 已连接：%s/%s", cfg.ClickHouseAddr, cfg.ClickHouseDatabase)
		}
	}

	r, auditWriter := router.SetupRouterWithAnalytics(pool, cfg, clickhouseConn)
	server := &http.Server{Addr: ":" + cfg.Port, Handler: r, ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("服务已启动，监听 %s", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalCtx.Done():
		log.Printf("收到退出信号，开始优雅关闭...")
	case serveErr := <-errCh:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Printf("HTTP 服务异常退出：%v", serveErr)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP 服务关闭超时：%v", err)
	}
	auditWriter.Stop()
	if clickhouseConn != nil {
		clickhouseConn.Close()
	}
}

// executeSchema 逐条执行建表/升级语句，ClickHouse HTTP 协议不接受多语句请求。
func executeSchema(ctx context.Context, conn clickhouse.Conn, schema string) error {
	for _, statement := range strings.Split(schema, ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if err := conn.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
