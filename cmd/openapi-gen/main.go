// openapi-gen 将 OpenAPI 3.x 文档（JSON/YAML）生成 Go MCP Server 骨架。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"MCP-Nexus/adapter/openapi"
)

func main() {
	in := flag.String("in", "", "OpenAPI 文档路径（JSON 或 YAML）")
	out := flag.String("out", "generated", "输出目录")
	module := flag.String("module", "generated-server", "生成的 Go module 名")
	flag.Parse()

	if *in == "" {
		fmt.Fprintln(os.Stderr, "用法: openapi-gen -in <spec> [-out <dir>] [-module <name>]")
		os.Exit(2)
	}

	data, err := os.ReadFile(*in)
	if err != nil {
		fatal(err)
	}
	spec, err := openapi.Parse(data)
	if err != nil {
		fatal(err)
	}
	if err := openapi.Validate(spec); err != nil {
		fatal(fmt.Errorf("OpenAPI 校验失败（存在不支持的特性）:\n%w", err))
	}
	tools, err := openapi.BuildTools(spec)
	if err != nil {
		fatal(err)
	}
	files, err := openapi.Generate(spec, tools, openapi.Options{ModuleName: *module})
	if err != nil {
		fatal(err)
	}

	for name, content := range files {
		path := filepath.Join(*out, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			fatal(err)
		}
		fmt.Printf("  wrote %s\n", path)
	}
	fmt.Printf("生成完成：%d 个工具 → %s\n", len(tools), *out)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "错误:", err)
	os.Exit(1)
}
