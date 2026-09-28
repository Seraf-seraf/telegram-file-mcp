package telegram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Seraf-seraf/telegram-file-mcp/internal/sendfile"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSendDocument(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/bottoken/sendDocument" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		fields := map[string]string{}
		for {
			p, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(p)
			if p.FormName() == "document" {
				if p.FileName() != "report.xlsx" || p.Header.Get("Content-Type") != "application/pdf" || string(b) != "bytes" {
					t.Errorf("file %s %q", p.FileName(), b)
				}
			} else {
				fields[p.FormName()] = string(b)
			}
		}
		if fields["chat_id"] != "fixed-chat" || fields["caption"] != "hello" {
			t.Errorf("fields=%v", fields)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":17}}`))
	}))
	defer server.Close()
	c := NewClient("token", "fixed-chat", server.Client(), server.URL)
	d, err := c.SendDocument(context.Background(), sendfile.File{Name: "report.xlsx", MIMEType: "application/pdf", Content: []byte("bytes")}, "hello")
	if err != nil || d.MessageID != 17 {
		t.Fatalf("delivery=%+v err=%v", d, err)
	}
}

func TestSendDocumentRejectionAndUnknown(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   sendfile.ErrorCategory
	}{
		{name: "http rejection", status: 400, body: `{"ok":false}`, want: sendfile.TelegramRejected},
		{name: "api rejection", status: 200, body: `{"ok":false}`, want: sendfile.TelegramRejected},
		{name: "ambiguous response", status: 200, body: `not-json`, want: sendfile.DeliveryUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := NewClient("sensitive-token", "chat", server.Client(), server.URL)
			_, err := client.SendDocument(context.Background(), sendfile.File{Name: "a", Content: []byte("x")}, "")
			if sendfile.CategoryOf(err) != tc.want || strings.Contains(err.Error(), "sensitive-token") {
				t.Fatalf("error=%v", err)
			}
		})
	}
	calls := 0
	client := NewClient("sensitive-token", "chat", &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("sensitive-token transport details")
	})}, "https://telegram.example")
	_, err := client.SendDocument(context.Background(), sendfile.File{Name: "a", Content: []byte("x")}, "")
	if sendfile.CategoryOf(err) != sendfile.DeliveryUnknown || calls != 1 || strings.Contains(err.Error(), "sensitive-token") {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}
