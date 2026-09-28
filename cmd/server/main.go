package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Seraf-seraf/telegram-file-mcp/internal/app"
	"github.com/Seraf-seraf/telegram-file-mcp/internal/config"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatal("invalid service configuration")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	mux := http.NewServeMux()
	handler := newAppHandler(cfg)
	mux.Handle("/api/mcp", handler)
	mux.Handle("/api/mcp/", handler)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func newAppHandler(cfg config.Config) http.Handler {
	if os.Getenv("VERCEL") == "1" {
		return app.NewForVercel(cfg)
	}
	return app.New(cfg)
}
