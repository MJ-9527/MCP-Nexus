// Skills → MCP 适配器示例
//
// 读取 skills.json，把其中声明的工具暴露为 Streamable HTTP 风格的 MCP 端点：
//
//	GET  /health
//	POST /tools/:name/call
//
// 产物可被注册到 MCP 网关，作为下游 MCP Server 被调用。
package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

//go:embed skills.json
var skillsJSON []byte

type skillSpec struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Category     string          `json:"category"`
	Tags         []string        `json:"tags"`
	InputSchema  json.RawMessage `json:"input_schema"`
	OutputSchema json.RawMessage `json:"output_schema"`
}

type skillsConfig struct {
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	Description string      `json:"description"`
	Endpoint    string      `json:"endpoint"`
	Tools       []skillSpec `json:"tools"`
}

var config skillsConfig
var tools map[string]skillSpec

func init() {
	if err := json.Unmarshal(skillsJSON, &config); err != nil {
		log.Fatalf("解析 skills.json 失败: %v", err)
	}
	tools = make(map[string]skillSpec, len(config.Tools))
	for _, t := range config.Tools {
		tools[t.Name] = t
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/tools/", handleTool)
	mux.HandleFunc("/.well-known/skills.json", handleSkillsManifest)

	log.Printf("[%s] listening on :%s, tools=%v", config.Name, port, toolNames())
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": config.Name,
		"version": config.Version,
	})
}

func handleSkillsManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(skillsJSON)
}

func handleTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/tools/")
	name = strings.TrimSuffix(name, "/call")
	spec, ok := tools[name]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "tool not found"})
		return
	}

	var args map[string]any
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}

	result, err := executeSkill(spec, args)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tool":   spec.Name,
		"result": result,
	})
}

func executeSkill(spec skillSpec, args map[string]any) (any, error) {
	switch spec.Name {
	case "calculate_vat":
		amount, ok := toFloat(args["amount"])
		if !ok {
			return nil, errors.New("amount must be a number")
		}
		rate := 0.13
		if r, ok := toFloat(args["rate"]); ok {
			rate = r
		}
		tax := amount * rate
		return map[string]any{
			"amount": amount,
			"tax":    round2(tax),
			"total":  round2(amount + tax),
		}, nil
	case "format_phone":
		phone, ok := args["phone"].(string)
		if !ok || len(phone) != 11 {
			return nil, errors.New("phone must be 11-digit string")
		}
		return map[string]any{
			"phone":     phone,
			"formatted": fmt.Sprintf("%s-%s-%s", phone[:3], phone[3:7], phone[7:]),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported skill %q", spec.Name)
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	}
	return 0, false
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

func toolNames() []string {
	out := make([]string, 0, len(tools))
	for _, t := range config.Tools {
		out = append(out, t.Name)
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
