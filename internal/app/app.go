package app

import (
	"net/http"
	"time"

	"github.com/Seraf-seraf/telegram-file-mcp/internal/config"
	"github.com/Seraf-seraf/telegram-file-mcp/internal/filefetch"
	"github.com/Seraf-seraf/telegram-file-mcp/internal/mcpserver"
	"github.com/Seraf-seraf/telegram-file-mcp/internal/sendfile"
	"github.com/Seraf-seraf/telegram-file-mcp/internal/telegram"
)

const (
	fileDownloadTimeout    = 15 * time.Second
	telegramRequestTimeout = 30 * time.Second
)

func New(cfg config.Config) http.Handler {
	fetcher := filefetch.NewClient(&http.Client{Timeout: fileDownloadTimeout}, cfg.MaxFileBytes)
	sender := telegram.NewClient(cfg.TelegramBotToken, cfg.TelegramChatID, &http.Client{Timeout: telegramRequestTimeout}, "")
	return mcpserver.NewHandler(sendfile.NewService(fetcher, sender))
}
