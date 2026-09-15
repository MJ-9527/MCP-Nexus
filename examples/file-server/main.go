package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// file-server：演示文件类 MCP Server。
// 工具：read_file（读取文件片段）、list_dir（列出目录）。
// 访问根目录通过环境变量 MCP_FILE_ROOT 限定（默认 /tmp）。

func main() {
	root := os.Getenv("MCP_FILE_ROOT")
	if root == "" {
		root = "/tmp"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/tools/read_file/call", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Arguments map[string]string `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		rel := req.Arguments["path"]
		if rel == "" {
			http.Error(w, "missing path", 400)
			return
		}
		// 防 path traversal：clean 后必须在 root 下
		full := filepath.Join(root, filepath.Clean("/"+rel))
		if !strings.HasPrefix(full, root) {
			http.Error(w, "path escapes root", 403)
			return
		}
		f, err := os.Open(full)
		if err != nil {
			respondMCP(w, true, fmt.Sprintf("打开文件失败: %v", err))
			return
		}
		defer f.Close()
		data := make([]byte, 4096)
		n, _ := f.Read(data)
		respondMCP(w, false, string(data[:n]))
	})
	mux.HandleFunc("/tools/list_dir/call", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Arguments map[string]string `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		rel := req.Arguments["dir"]
		if rel == "" {
			rel = "/"
		}
		full := filepath.Join(root, filepath.Clean("/"+rel))
		if !strings.HasPrefix(full, root) {
			http.Error(w, "path escapes root", 403)
			return
		}
		entries, err := os.ReadDir(full)
		if err != nil {
			respondMCP(w, true, fmt.Sprintf("读取目录失败: %v", err))
			return
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		out, _ := json.Marshal(names)
		respondMCP(w, false, string(out))
	})

	addr := ":9002"
	fmt.Println("file-server listening on", addr)
	http.ListenAndServe(addr, mux)
}

func respondMCP(w io.Writer, isError bool, text string) {
	out, _ := json.Marshal(map[string]interface{}{
		"content":  []map[string]string{{"type": "text", "text": text}},
		"is_error": isError,
	})
	w.Write(out)
}
