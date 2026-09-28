package config

import "testing"

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		values  map[string]string
		want    int64
		wantErr bool
	}{
		{name: "default", values: map[string]string{"TELEGRAM_BOT_TOKEN": "token", "TELEGRAM_CHAT_ID": "chat"}, want: DefaultMaxFileBytes},
		{name: "custom", values: map[string]string{"TELEGRAM_BOT_TOKEN": "token", "TELEGRAM_CHAT_ID": "chat", "MAX_FILE_BYTES": "123"}, want: 123},
		{name: "empty token", values: map[string]string{"TELEGRAM_CHAT_ID": "chat"}, wantErr: true},
		{name: "empty chat", values: map[string]string{"TELEGRAM_BOT_TOKEN": "token"}, wantErr: true},
		{name: "zero", values: map[string]string{"TELEGRAM_BOT_TOKEN": "t", "TELEGRAM_CHAT_ID": "c", "MAX_FILE_BYTES": "0"}, wantErr: true},
		{name: "negative", values: map[string]string{"TELEGRAM_BOT_TOKEN": "t", "TELEGRAM_CHAT_ID": "c", "MAX_FILE_BYTES": "-1"}, wantErr: true},
		{name: "maximum allowed", values: map[string]string{"TELEGRAM_BOT_TOKEN": "t", "TELEGRAM_CHAT_ID": "c", "MAX_FILE_BYTES": "52428800"}, want: 52428800},
		{name: "above max", values: map[string]string{"TELEGRAM_BOT_TOKEN": "t", "TELEGRAM_CHAT_ID": "c", "MAX_FILE_BYTES": "52428801"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(func(key string) string { return tt.values[key] })
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v", err)
			}
			if err == nil && cfg.MaxFileBytes != tt.want {
				t.Fatalf("MaxFileBytes = %d, want %d", cfg.MaxFileBytes, tt.want)
			}
		})
	}
}
