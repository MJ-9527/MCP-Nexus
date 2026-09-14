package client

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type HealthClient struct{ httpClient *http.Client }

func NewHealthClient(timeout time.Duration) *HealthClient {
	return &HealthClient{httpClient: &http.Client{Timeout: timeout}}
}
func (c *HealthClient) Check(ctx context.Context, endpoint string) (time.Duration, int, error) {
	url := strings.TrimRight(endpoint, "/") + "/health"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, err
	}
	startedAt := time.Now()
	response, err := c.httpClient.Do(request)
	latency := time.Since(startedAt)
	if err != nil {
		return latency, 0, err
	}
	defer response.Body.Close()
	return latency, response.StatusCode, nil
}
