package sendfile

import (
	"context"
	"testing"
)

type fakeFetcher struct {
	file  File
	err   error
	calls int
	ctx   context.Context
}

func (f *fakeFetcher) Fetch(ctx context.Context, _ FileSource) (File, error) {
	f.calls++
	f.ctx = ctx
	return f.file, f.err
}

type fakeSender struct {
	delivery Delivery
	err      error
	calls    int
	ctx      context.Context
}

func (s *fakeSender) SendDocument(ctx context.Context, _ File, _ string) (Delivery, error) {
	s.calls++
	s.ctx = ctx
	return s.delivery, s.err
}
func TestServiceSendAndErrors(t *testing.T) {
	f := &fakeFetcher{file: File{Name: "report.xlsx", SizeBytes: 7}}
	s := &fakeSender{delivery: Delivery{MessageID: 42}}
	service := NewService(f, s)
	ctx := context.WithValue(context.Background(), struct{}{}, "marker")
	result, err := service.Send(ctx, Request{Source: FileSource{DownloadURL: "https://example.test/file"}})
	if err != nil || result.MessageID != 42 || result.FileName != "report.xlsx" || result.SizeBytes != 7 || f.calls != 1 || s.calls != 1 || f.ctx != ctx || s.ctx != ctx {
		t.Fatalf("result=%+v err=%v fetch=%d send=%d", result, err, f.calls, s.calls)
	}
	f.err = NewError(DownloadFailed)
	s.calls = 0
	_, err = service.Send(ctx, Request{Source: FileSource{DownloadURL: "https://example.test/a"}})
	if CategoryOf(err) != DownloadFailed || s.calls != 0 {
		t.Fatalf("err=%v send calls=%d", err, s.calls)
	}
}
func TestCaptionLimit(t *testing.T) {
	f := &fakeFetcher{file: File{Name: "x"}}
	s := &fakeSender{}
	service := NewService(f, s)
	if _, err := service.Send(context.Background(), Request{Source: FileSource{DownloadURL: "https://example.test/a"}, Caption: string(make([]rune, 1024))}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Send(context.Background(), Request{Source: FileSource{DownloadURL: "https://example.test/a"}, Caption: string(make([]rune, 1025))}); CategoryOf(err) != InvalidInput || f.calls != 1 || s.calls != 1 {
		t.Fatalf("expected pre-side-effect invalid_input, got %v", err)
	}
}

func TestSenderErrorsAreReturnedWithoutRetry(t *testing.T) {
	for _, category := range []ErrorCategory{TelegramRejected, DeliveryUnknown} {
		fetcher := &fakeFetcher{file: File{Name: "x"}}
		sender := &fakeSender{err: NewError(category)}
		service := NewService(fetcher, sender)
		_, err := service.Send(context.Background(), Request{Source: FileSource{DownloadURL: "https://example.test/a"}})
		if CategoryOf(err) != category || sender.calls != 1 {
			t.Fatalf("category=%s err=%v calls=%d", category, err, sender.calls)
		}
	}
}
