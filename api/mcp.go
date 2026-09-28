package handler

import (
	"net/http"
	"sync"

	"github.com/Seraf-seraf/telegram-file-mcp/internal/app"
	"github.com/Seraf-seraf/telegram-file-mcp/internal/config"
)

var (
	once       sync.Once
	mcpHandler http.Handler
	configErr  error
)

func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		var cfg config.Config
		cfg, configErr = config.LoadFromEnv()
		if configErr == nil {
			mcpHandler = app.NewForVercel(cfg)
		}
	})
	if configErr != nil {
		http.Error(w, "service configuration error", http.StatusInternalServerError)
		return
	}
	mcpHandler.ServeHTTP(w, r)
}
