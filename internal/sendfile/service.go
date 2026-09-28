package sendfile

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

type ErrorCategory string

const (
	InvalidInput     ErrorCategory = "invalid_input"
	FileTooLarge     ErrorCategory = "file_too_large"
	DownloadFailed   ErrorCategory = "download_failed"
	TelegramRejected ErrorCategory = "telegram_rejected"
	DeliveryUnknown  ErrorCategory = "delivery_unknown"
)

type Error struct{ Category ErrorCategory }

func (e *Error) Error() string              { return string(e.Category) }
func NewError(category ErrorCategory) error { return &Error{Category: category} }
func CategoryOf(err error) ErrorCategory {
	var categorized *Error
	if errors.As(err, &categorized) {
		return categorized.Category
	}
	return InvalidInput
}

type FileSource struct {
	DownloadURL string
	FileName    string
	MIMEType    string
}
type File struct {
	Name      string
	MIMEType  string
	SizeBytes int64
	Content   []byte
}
type Request struct {
	Source  FileSource
	Caption string
}
type Delivery struct{ MessageID int64 }
type Result struct {
	Status    string `json:"status"`
	MessageID int64  `json:"message_id"`
	FileName  string `json:"file_name"`
	SizeBytes int64  `json:"size_bytes"`
}
type FileFetcher interface {
	Fetch(context.Context, FileSource) (File, error)
}
type DocumentSender interface {
	SendDocument(context.Context, File, string) (Delivery, error)
}
type Service struct {
	fetcher FileFetcher
	sender  DocumentSender
}

func NewService(fetcher FileFetcher, sender DocumentSender) *Service {
	return &Service{fetcher: fetcher, sender: sender}
}
func (s *Service) Send(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.Source.DownloadURL) == "" || utf8.RuneCountInString(req.Caption) > 1024 {
		return Result{}, NewError(InvalidInput)
	}
	file, err := s.fetcher.Fetch(ctx, req.Source)
	if err != nil {
		return Result{}, err
	}
	delivery, err := s.sender.SendDocument(ctx, file, req.Caption)
	if err != nil {
		return Result{}, err
	}
	return Result{Status: "sent", MessageID: delivery.MessageID, FileName: file.Name, SizeBytes: file.SizeBytes}, nil
}
