package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Seraf-seraf/telegram-file-mcp/internal/config"
)

func TestNewAppHandlerSelectsHostProtectionByRuntime(t *testing.T) {
	for _, test := range []struct {
		name       string
		vercel     string
		wantStatus int
	}{
		{name: "local runtime keeps localhost protection", wantStatus: http.StatusForbidden},
		{name: "vercel runtime accepts forwarded host", vercel: "1", wantStatus: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("VERCEL", test.vercel)
			handler := newAppHandler(config.Config{TelegramBotToken: "test", TelegramChatID: "test", MaxFileBytes: 1024})
			server := httptest.NewServer(handler)
			defer server.Close()

			body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
			request, err := http.NewRequest(http.MethodPost, server.URL, bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			request.Host = "telegram-file-mcp.vercel.app"

			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status=%d, want %d", response.StatusCode, test.wantStatus)
			}
		})
	}
}
