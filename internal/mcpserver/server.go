package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Seraf-seraf/telegram-file-mcp/internal/sendfile"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Sender interface {
	Send(context.Context, sendfile.Request) (sendfile.Result, error)
}
type schemaString string

func (s *schemaString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("expected string")
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*s = schemaString(value)
	return nil
}

type arguments struct {
	File struct {
		DownloadURL schemaString `json:"download_url"`
		FileID      schemaString `json:"file_id"`
		MIMEType    schemaString `json:"mime_type,omitempty"`
		FileName    schemaString `json:"file_name,omitempty"`
	} `json:"file"`
	Caption schemaString `json:"caption,omitempty"`
}

func NewHandler(service Sender, disableLocalhostProtection bool) http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "telegram-file-mcp", Version: "0.1.0"}, nil)
	falseValue := false
	trueValue := true
	tool := &mcp.Tool{
		Name:         "send_file_to_telegram",
		Description:  "Отправляет файл в заранее настроенный Telegram-чат. Получатель не выбирается моделью. Используйте инструмент только после явного запроса пользователя отправить файл в Telegram.",
		Meta:         mcp.Meta{"openai/fileParams": []string{"file"}},
		Annotations:  &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &falseValue, IdempotentHint: false, OpenWorldHint: &trueValue},
		InputSchema:  json.RawMessage(`{"type":"object","properties":{"file":{"type":"object","properties":{"download_url":{"type":"string"},"file_id":{"type":"string"},"mime_type":{"type":"string"},"file_name":{"type":"string"}},"required":["download_url","file_id"],"additionalProperties":false},"caption":{"type":"string"}},"required":["file"],"additionalProperties":false}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"status":{"type":"string","const":"sent"},"message_id":{"type":"integer"},"file_name":{"type":"string"},"size_bytes":{"type":"integer","minimum":0}},"required":["status","message_id","file_name","size_bytes"],"additionalProperties":false}`),
	}
	server.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input arguments
		decoder := json.NewDecoder(bytes.NewReader(req.Params.Arguments))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || strings.TrimSpace(string(input.File.DownloadURL)) == "" || strings.TrimSpace(string(input.File.FileID)) == "" {
			return toolError(sendfile.InvalidInput), nil
		}
		result, err := service.Send(ctx, sendfile.Request{Source: sendfile.FileSource{DownloadURL: string(input.File.DownloadURL), FileName: string(input.File.FileName), MIMEType: string(input.File.MIMEType)}, Caption: string(input.Caption)})
		if err != nil {
			return toolError(sendfile.CategoryOf(err)), nil
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return toolError(sendfile.InvalidInput), nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}, StructuredContent: result}, nil
	})
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 1 << 20, DisableLocalhostProtection: disableLocalhostProtection})
}
func toolError(category sendfile.ErrorCategory) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(category)}}}
}
