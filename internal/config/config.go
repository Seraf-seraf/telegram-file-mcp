package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultMaxFileBytes int64 = 20 * 1024 * 1024
	MaxAllowedFileBytes int64 = 50 * 1024 * 1024
)

type Config struct {
	TelegramBotToken string
	TelegramChatID   string
	MaxFileBytes     int64
}

func LoadFromEnv() (Config, error) {
	return Load(os.Getenv)
}

func Load(getenv func(string) string) (Config, error) {
	cfg := Config{TelegramBotToken: strings.TrimSpace(getenv("TELEGRAM_BOT_TOKEN")), TelegramChatID: strings.TrimSpace(getenv("TELEGRAM_CHAT_ID")), MaxFileBytes: DefaultMaxFileBytes}
	if cfg.TelegramBotToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.TelegramChatID == "" {
		return Config{}, fmt.Errorf("TELEGRAM_CHAT_ID is required")
	}
	if raw := strings.TrimSpace(getenv("MAX_FILE_BYTES")); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 || n > MaxAllowedFileBytes {
			return Config{}, fmt.Errorf("MAX_FILE_BYTES must be between 1 and %d", MaxAllowedFileBytes)
		}
		cfg.MaxFileBytes = n
	}
	return cfg, nil
}
