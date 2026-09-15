package middleware

import (
	"fmt"
	"strconv"
	"time"

	"MCP-Nexus/model"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// defaultQPS 默认限流（每分钟请求数），用于未显式配置的角色。
const defaultQPS = 60

// RateLimit 按角色限流中间件（Redis 固定窗口，分钟桶）。
// cfg: role -> 每分钟最大请求数。
func RateLimit(rdb *redis.Client, cfg map[string]int) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := GetRole(c)
		limit := cfg[role]
		if limit <= 0 {
			limit = defaultQPS
		}
		now := time.Now()
		bucket := now.Truncate(time.Minute).Unix()
		key := fmt.Sprintf("ratelimit:role:%s:%d", role, bucket)

		ctx := c.Request.Context()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Redis 异常时降级放行，避免限流组件故障导致全站不可用
			c.Next()
			return
		}
		if count == 1 {
			rdb.Expire(ctx, key, time.Minute)
		}
		if count > int64(limit) {
			c.Header("Retry-After", strconv.Itoa(60-now.Second()))
			c.AbortWithStatusJSON(429, model.APIResponse{
				Code:      "RATE_LIMITED",
				Message:   fmt.Sprintf("角色 %s 触发限流（每分钟上限 %d）", role, limit),
				RequestID: c.GetString("request_id"),
			})
			return
		}
		c.Next()
	}
}
