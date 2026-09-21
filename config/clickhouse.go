package config

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"time"
)

func NewClickHouse(ctx context.Context, cfg Config) (clickhouse.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{Addr: []string{cfg.ClickHouseAddr}, Protocol: clickhouse.HTTP, Auth: clickhouse.Auth{Database: cfg.ClickHouseDatabase, Username: cfg.ClickHouseUser, Password: cfg.ClickHousePassword}, DialTimeout: 5 * time.Second})
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
