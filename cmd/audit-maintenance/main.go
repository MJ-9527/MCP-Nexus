package main

import (
	"MCP-Nexus/config"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"
	"context"
	"flag"
	"fmt"
	"log"
)

func main() {
	online := flag.Int("online-days", 90, "days retained in audit_logs")
	archive := flag.Int("archive-days", 365, "days retained in audit_logs_archive")
	batch := flag.Int("batch-size", 1000, "maximum rows per transaction")
	execute := flag.Bool("execute", false, "perform changes; default is preview only")
	flag.Parse()
	config.Load()
	ctx := context.Background()
	pool, e := config.NewPostgresPool(ctx)
	if e != nil {
		log.Fatal(e)
	}
	defer pool.Close()
	svc := service.NewAuditRetentionService(repository.NewPostgresAuditRetentionRepository(pool))
	preview, e := svc.Preview(ctx, *online, *archive)
	if e != nil {
		log.Fatal(e)
	}
	fmt.Printf("preview archive_rows=%d delete_rows=%d archive_before=%s delete_before=%s\n", preview.ArchiveRows, preview.DeleteRows, preview.ArchiveBefore.Format("2006-01-02"), preview.DeleteBefore.Format("2006-01-02"))
	if !*execute {
		return
	}
	result, e := svc.Run(ctx, *online, *archive, *batch)
	if e != nil {
		log.Fatal(e)
	}
	fmt.Printf("completed archived_rows=%d deleted_rows=%d\n", result.ArchivedRows, result.DeletedRows)
}
