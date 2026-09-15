package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// db-server：演示数据库类 MCP Server。
// 工具：query_sales（返回脱敏销售数据）、query_inventory（返回库存数据）。
// 内置静态数据，模拟真实 DB 查询。

var salesData = []map[string]interface{}{
	{"region": "华东", "month": "2026-08", "amount": 123400, "customer_count": 56},
	{"region": "华北", "month": "2026-08", "amount": 98700, "customer_count": 43},
	{"region": "华南", "month": "2026-08", "amount": 156200, "customer_count": 78},
}

var inventoryData = []map[string]interface{}{
	{"sku": "SKU-001", "name": "商品A", "stock": 120, "warehouse": "深圳仓"},
	{"sku": "SKU-002", "name": "商品B", "stock": 45, "warehouse": "上海仓"},
	{"sku": "SKU-003", "name": "商品C", "stock": 0, "warehouse": "北京仓"},
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/tools/query_sales/call", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Arguments map[string]string `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		month := req.Arguments["month"]
		out, _ := json.Marshal(salesData)
		respondMCP(w, false, fmt.Sprintf("月度销售数据(查询月份=%s): %s", month, string(out)))
	})
	mux.HandleFunc("/tools/query_inventory/call", func(w http.ResponseWriter, r *http.Request) {
		out, _ := json.Marshal(inventoryData)
		respondMCP(w, false, "库存数据: "+string(out))
	})

	addr := ":9004"
	fmt.Println("db-server listening on", addr)
	http.ListenAndServe(addr, mux)
}

func respondMCP(w io.Writer, isError bool, text string) {
	out, _ := json.Marshal(map[string]interface{}{
		"content":  []map[string]string{{"type": "text", "text": text}},
		"is_error": isError,
	})
	w.Write(out)
}
