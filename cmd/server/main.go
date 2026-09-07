package main

import (
	"MCP-Nexus/config"
	"MCP-Nexus/router"
	"log"
)

func main() {
	cfg := config.Load()

	r := router.SetupRouter()
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}

}
