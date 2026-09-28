package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"time"

	"github.com/Seraf-seraf/telegram-file-mcp/internal/sendfile"
)

const defaultBaseURL = "https://api.telegram.org"

type Client struct {
	token, chatID string
	httpClient    *http.Client
	baseURL       string
}

func NewClient(token, chatID string, httpClient *http.Client, baseURL string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{token: token, chatID: chatID, httpClient: httpClient, baseURL: baseURL}
}
func (c *Client) SendDocument(ctx context.Context, file sendfile.File, caption string) (sendfile.Delivery, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("chat_id", c.chatID)
	if caption != "" {
		_ = writer.WriteField("caption", caption)
	}
	contentType := "application/octet-stream"
	if file.MIMEType != "" {
		if _, _, parseErr := mime.ParseMediaType(file.MIMEType); parseErr != nil {
			return sendfile.Delivery{}, sendfile.NewError(sendfile.InvalidInput)
		}
		contentType = file.MIMEType
	}
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "document", "filename": file.Name}))
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return sendfile.Delivery{}, sendfile.NewError(sendfile.InvalidInput)
	}
	if _, err = part.Write(file.Content); err != nil {
		return sendfile.Delivery{}, sendfile.NewError(sendfile.InvalidInput)
	}
	if err = writer.Close(); err != nil {
		return sendfile.Delivery{}, sendfile.NewError(sendfile.InvalidInput)
	}
	endpoint, err := url.JoinPath(c.baseURL, "bot"+c.token, "sendDocument")
	if err != nil {
		return sendfile.Delivery{}, sendfile.NewError(sendfile.InvalidInput)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return sendfile.Delivery{}, sendfile.NewError(sendfile.DeliveryUnknown)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return sendfile.Delivery{}, sendfile.NewError(sendfile.DeliveryUnknown)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return sendfile.Delivery{}, sendfile.NewError(sendfile.TelegramRejected)
	}
	limited, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return sendfile.Delivery{}, sendfile.NewError(sendfile.DeliveryUnknown)
	}
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	if json.Unmarshal(limited, &result) != nil {
		return sendfile.Delivery{}, sendfile.NewError(sendfile.DeliveryUnknown)
	}
	if !result.OK {
		return sendfile.Delivery{}, sendfile.NewError(sendfile.TelegramRejected)
	}
	if result.Result.MessageID <= 0 {
		return sendfile.Delivery{}, sendfile.NewError(sendfile.DeliveryUnknown)
	}
	return sendfile.Delivery{MessageID: result.Result.MessageID}, nil
}
