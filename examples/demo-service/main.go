// demo-service 模拟下游 MCP Server：提供 query_sales / query_inventory 两个工具。
// 网关契约：POST {endpoint}，请求体 {"method":"tools/call","toolName":"...","arguments":{...}}，
// 响应体 {"content":[{"type":"text","text":"..."}],"is_error":false}。
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

type callRequest struct {
	Method    string                 `json:"method"`
	ToolName  string                 `json:"toolName"`
	Arguments map[string]interface{} `json:"arguments"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type callResponse struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"is_error,omitempty"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", handleCall)
	mux.HandleFunc("/tools/", handleCall)

	addr := os.Getenv("DEMO_ADDR")
	if addr == "" {
		addr = ":9001"
	}
	log.Printf("demo-service listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, callResponse{IsError: true, Content: []contentItem{{Type: "text", Text: "method not allowed"}}})
		return
	}
	var req callRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, callResponse{IsError: true, Content: []contentItem{{Type: "text", Text: "invalid json"}}})
		return
	}
	// 兼容路径式调用：/tools/query_sales/call
	if req.ToolName == "" {
		req.ToolName = extractToolName(r.URL.Path)
	}

	var text string
	switch req.ToolName {
	case "query_sales":
		text = querySales(req.Arguments)
	case "query_inventory":
		text = queryInventory(req.Arguments)
	case "":
		writeJSON(w, http.StatusOK, callResponse{IsError: true, Content: []contentItem{{Type: "text", Text: "toolName is required"}}})
		return
	default:
		writeJSON(w, http.StatusOK, callResponse{IsError: true, Content: []contentItem{{Type: "text", Text: fmt.Sprintf("unknown tool: %s", req.ToolName)}}})
		return
	}
	writeJSON(w, http.StatusOK, callResponse{Content: []contentItem{{Type: "text", Text: text}}})
}

func extractToolName(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "tools" {
		return parts[1]
	}
	return ""
}

func querySales(args map[string]interface{}) string {
	month, _ := args["month"].(string)
	if month == "" {
		month = "2026-08"
	}
	region, _ := args["region"].(string)
	if region == "" {
		region = "全国"
	}
	return fmt.Sprintf(`{"month":"%s","region":"%s","total_sales":1285000.50,"order_count":342,"top_product":"智能网关Pro","yoy_growth":0.18}`, month, region)
}

func queryInventory(args map[string]interface{}) string {
	sku, _ := args["sku"].(string)
	if sku == "" {
		sku = "ALL"
	}
	return fmt.Sprintf(`{"sku":"%s","in_stock":5230,"reserved":180,"warning_threshold":500,"warehouses":["华东仓","华南仓"]}`, sku)
}

func writeJSON(w http.ResponseWriter, status int, resp callResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
