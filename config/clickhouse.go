package config

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func NewClickHouse(ctx context.Context, cfg Config) (clickhouse.Conn, error) {
	addr := strings.TrimPrefix(strings.TrimPrefix(cfg.ClickHouseAddr, "http://"), "https://")
	if addr == "" {
		return nil, fmt.Errorf("clickhouse addr is empty")
	}
	conn, err := clickhouse.Open(&clickhouse.Options{Addr: []string{addr}, Protocol: clickhouse.HTTP, Auth: clickhouse.Auth{Database: cfg.ClickHouseDatabase, Username: cfg.ClickHouseUser, Password: cfg.ClickHousePassword}, DialTimeout: 5 * time.Second})
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.Ping(pingCtx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping clickhouse %s: %w", cfg.ClickHouseAddr, err)
	}
	return conn, nil
}
