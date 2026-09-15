package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// http-server：演示 HTTP 转发类 MCP Server。
// 工具：http_get（转发 GET 请求）、http_post（转发 POST 请求）。
// 为防 SSRF，只允许白名单域名（example.com / httpbin.org）。

var allowedHosts = map[string]bool{
	"example.com": true,
	"httpbin.org": true,
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/tools/http_get/call", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Arguments map[string]string `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		target := req.Arguments["url"]
		if !isAllowed(target) {
			respondMCP(w, true, "域名不在白名单")
			return
		}
		resp, err := http.Get(target)
		if err != nil {
			respondMCP(w, true, fmt.Sprintf("请求失败: %v", err))
			return
		}
		defer resp.Body.Close()
		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		respondMCP(w, false, fmt.Sprintf("status=%d body=%s", resp.StatusCode, string(body[:n])))
	})
	mux.HandleFunc("/tools/http_post/call", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Arguments map[string]string `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		target := req.Arguments["url"]
		body := req.Arguments["body"]
		if !isAllowed(target) {
			respondMCP(w, true, "域名不在白名单")
			return
		}
		resp, err := http.Post(target, "application/json", strings.NewReader(body))
		if err != nil {
			respondMCP(w, true, fmt.Sprintf("请求失败: %v", err))
			return
		}
		defer resp.Body.Close()
		respBody := make([]byte, 4096)
		n, _ := resp.Body.Read(respBody)
		respondMCP(w, false, fmt.Sprintf("status=%d body=%s", resp.StatusCode, string(respBody[:n])))
	})

	addr := ":9003"
	fmt.Println("http-server listening on", addr)
	http.ListenAndServe(addr, mux)
}

func isAllowed(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return allowedHosts[u.Hostname()]
}

func respondMCP(w io.Writer, isError bool, text string) {
	out, _ := json.Marshal(map[string]interface{}{
		"content":  []map[string]string{{"type": "text", "text": text}},
		"is_error": isError,
	})
	w.Write(out)
}
