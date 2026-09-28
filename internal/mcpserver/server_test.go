package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Seraf-seraf/telegram-file-mcp/internal/sendfile"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeSender struct {
	calls   int
	request sendfile.Request
}

func (f *fakeSender) Send(_ context.Context, r sendfile.Request) (sendfile.Result, error) {
	f.calls++
	f.request = r
	return sendfile.Result{Status: "sent", MessageID: 4, FileName: "x", SizeBytes: 1}, nil
}
func TestMCPTool(t *testing.T) {
	f := &fakeSender{}
	httpServer := httptest.NewServer(NewHandler(f, false))
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != 1 || tools.Tools[0].Name != "send_file_to_telegram" {
		t.Fatalf("tools=%+v err=%v", tools, err)
	}
	tool := tools.Tools[0]
	if tool.Meta["openai/fileParams"] == nil || tool.Annotations == nil || tool.Annotations.ReadOnlyHint || *tool.Annotations.DestructiveHint || tool.Annotations.IdempotentHint || !*tool.Annotations.OpenWorldHint {
		t.Fatalf("tool metadata or annotations violate contract: %+v", tool)
	}
	inputSchema, _ := json.Marshal(tool.InputSchema)
	for _, field := range []string{`"download_url"`, `"file_id"`, `"mime_type"`, `"file_name"`, `"chat_id"`} {
		if field == `"chat_id"` {
			if bytes.Contains(inputSchema, []byte(field)) {
				t.Fatal("chat_id must not be an input")
			}
			continue
		}
		if !bytes.Contains(inputSchema, []byte(field)) {
			t.Fatalf("input schema missing %s", field)
		}
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "send_file_to_telegram", Arguments: map[string]any{"file": map[string]any{"download_url": "https://example.test/a", "file_id": "id"}}})
	if err != nil || result.IsError || f.calls != 1 || f.request.Source.DownloadURL != "https://example.test/a" {
		t.Fatalf("result=%+v err=%v calls=%d", result, err, f.calls)
	}
	if len(result.Content) != 1 {
		t.Fatalf("successful result has no content: %+v", result)
	}
	output, ok := result.StructuredContent.(map[string]any)
	if !ok || output["status"] != "sent" || output["message_id"] != float64(4) || output["file_name"] != "x" || output["size_bytes"] != float64(1) {
		t.Fatalf("structured output=%#v", result.StructuredContent)
	}
	invalid, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "send_file_to_telegram", Arguments: map[string]any{"file": map[string]any{"download_url": "https://example.test/a", "file_id": "id"}, "caption": nil}})
	if err != nil || !invalid.IsError || f.calls != 1 {
		t.Fatalf("null string input was accepted: result=%+v err=%v", invalid, err)
	}
	_, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "send_file_to_telegram", Arguments: map[string]any{"file": map[string]any{"download_url": "https://example.test/a", "file_id": "id"}}})
	if err != nil || f.calls != 2 {
		t.Fatalf("repeated explicit call was deduplicated: calls=%d err=%v", f.calls, err)
	}
	response, err := http.Post(httpServer.URL, "application/json", bytes.NewReader(append([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`), bytes.Repeat([]byte(" "), 1<<20)...)))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode < 400 {
		t.Fatalf("oversized request status=%d", response.StatusCode)
	}
}

func TestLocalhostProtectionCanBeDisabledForVercelProxy(t *testing.T) {
	for _, test := range []struct {
		name                       string
		disableLocalhostProtection bool
		wantStatus                 int
	}{
		{name: "local server rejects forwarded public host", wantStatus: http.StatusForbidden},
		{name: "vercel handler accepts forwarded public host", disableLocalhostProtection: true, wantStatus: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(NewHandler(&fakeSender{}, test.disableLocalhostProtection))
			defer server.Close()

			requestBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
			request, err := http.NewRequest(http.MethodPost, server.URL, bytes.NewReader(requestBody))
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
