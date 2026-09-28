package filefetch

import (
	"context"
	"io"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/Seraf-seraf/telegram-file-mcp/internal/sendfile"
)

type testResolver struct{ ips []netip.Addr }

func (r testResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r.ips, nil
}

type mappingResolver map[string][]netip.Addr

func (r mappingResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	return r[host], nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFetchSizeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name          string
		body          string
		contentLength int64
		category      sendfile.ErrorCategory
	}{
		{name: "below limit", body: "123", contentLength: 3},
		{name: "at limit", body: "1234", contentLength: 4},
		{name: "above limit", body: "12345", contentLength: 5, category: sendfile.FileTooLarge},
		{name: "missing content length", body: "12345", contentLength: -1, category: sendfile.FileTooLarge},
		{name: "false small content length", body: "12345", contentLength: 1, category: sendfile.FileTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.body)), ContentLength: tc.contentLength}, nil
			})
			client := NewClient(&http.Client{Transport: transport}, 4)
			client.resolver = testResolver{[]netip.Addr{netip.MustParseAddr("8.8.8.8")}}
			file, err := client.Fetch(context.Background(), sendfile.FileSource{DownloadURL: "https://example.test/file", FileName: "../../report.xlsx"})
			if tc.category != "" {
				if sendfile.CategoryOf(err) != tc.category {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil || file.Name != "report.xlsx" || file.SizeBytes != int64(len(tc.body)) || string(file.Content) != tc.body {
				t.Fatalf("file=%+v err=%v", file, err)
			}
		})
	}
}

func TestFetchCancellationReturnsDownloadFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		cancel()
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	client := NewClient(&http.Client{Transport: transport}, 4)
	client.resolver = testResolver{[]netip.Addr{netip.MustParseAddr("8.8.8.8")}}
	_, err := client.Fetch(ctx, sendfile.FileSource{DownloadURL: "https://example.test/file"})
	if sendfile.CategoryOf(err) != sendfile.DownloadFailed || calls != 1 {
		t.Fatalf("error=%v requests=%d", err, calls)
	}
}

func TestFetchFallbackName(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("x")), ContentLength: 1}, nil
	})}, 4)
	client.resolver = testResolver{[]netip.Addr{netip.MustParseAddr("8.8.8.8")}}
	file, err := client.Fetch(context.Background(), sendfile.FileSource{DownloadURL: "https://example.test/"})
	if err != nil || file.Name != "document" {
		t.Fatalf("file=%+v err=%v", file, err)
	}
}

func TestRejectUnsafeURLs(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { panic("request should not run") })}, 4)
	client.resolver = testResolver{[]netip.Addr{netip.MustParseAddr("100.64.0.1")}}
	for _, raw := range []string{"http://example.test/a", "https://localhost/a", "https://example.test/a"} {
		if _, err := client.Fetch(context.Background(), sendfile.FileSource{DownloadURL: raw}); sendfile.CategoryOf(err) != sendfile.InvalidInput {
			t.Fatalf("URL %s error=%v", raw, err)
		}
	}
}

func TestRedirectTargetsAreValidated(t *testing.T) {
	publicIP := netip.MustParseAddr("8.8.8.8")
	privateIP := netip.MustParseAddr("10.0.0.4")
	for _, tc := range []struct {
		name         string
		redirectHost string
		redirectIP   netip.Addr
		wantRequestN int
		wantError    bool
	}{
		{name: "public redirect", redirectHost: "public.example", redirectIP: publicIP, wantRequestN: 2},
		{name: "private redirect", redirectHost: "private.example", redirectIP: privateIP, wantRequestN: 1, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					header := make(http.Header)
					header.Set("Location", "https://"+tc.redirectHost+"/file")
					return &http.Response{StatusCode: http.StatusFound, Header: header, Body: io.NopCloser(strings.NewReader("")), ContentLength: 0}, nil
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("file")), ContentLength: 4}, nil
			})
			client := NewClient(&http.Client{Transport: transport}, 4)
			client.resolver = mappingResolver{
				"start.example": {publicIP},
				tc.redirectHost: {tc.redirectIP},
			}
			_, err := client.Fetch(context.Background(), sendfile.FileSource{DownloadURL: "https://start.example/file"})
			if (err != nil) != tc.wantError || calls != tc.wantRequestN {
				t.Fatalf("err=%v requests=%d, want error=%t requests=%d", err, calls, tc.wantError, tc.wantRequestN)
			}
		})
	}
}

func TestRedirectLimit(t *testing.T) {
	for _, redirectCount := range []int{5, 6} {
		t.Run(strconv.Itoa(redirectCount), func(t *testing.T) {
			calls := 0
			transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				if calls <= redirectCount {
					header := make(http.Header)
					header.Set("Location", "https://redirect"+strconv.Itoa(calls)+".example/file")
					return &http.Response{StatusCode: http.StatusFound, Header: header, Body: io.NopCloser(strings.NewReader("")), ContentLength: 0}, nil
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok")), ContentLength: 2}, nil
			})
			client := NewClient(&http.Client{Transport: transport}, 4)
			client.resolver = testResolver{[]netip.Addr{netip.MustParseAddr("8.8.8.8")}}
			_, err := client.Fetch(context.Background(), sendfile.FileSource{DownloadURL: "https://start.example/file"})
			if redirectCount == 5 && (err != nil || calls != 6) {
				t.Fatalf("five redirects should succeed: err=%v requests=%d", err, calls)
			}
			if redirectCount == 6 && (err == nil || calls != 6) {
				t.Fatalf("six redirects should be rejected: err=%v requests=%d", err, calls)
			}
		})
	}
}

func TestCleanName(t *testing.T) {
	cases := map[string]string{"../../report.xlsx": "report.xlsx", "a\r\nb\x00.txt": "ab.txt", "": "", "..": ""}
	for input, want := range cases {
		if got := cleanName(input); got != want {
			t.Errorf("cleanName(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestPublicAddressFilterRejectsSpecialRanges(t *testing.T) {
	cases := []struct {
		address string
		want    bool
	}{
		{address: "8.8.8.8", want: true},
		{address: "2001:4860:4860::8888", want: true},
		{address: "192.0.2.1"},
		{address: "198.51.100.1"},
		{address: "203.0.113.1"},
		{address: "198.18.0.1"},
		{address: "192.0.0.1"},
		{address: "2001:db8::1"},
		{address: "3fff::1"},
		{address: "100::1"},
	}
	for _, tc := range cases {
		t.Run(tc.address, func(t *testing.T) {
			if got := isPublic(netip.MustParseAddr(tc.address)); got != tc.want {
				t.Fatalf("isPublic(%s)=%t, want %t", tc.address, got, tc.want)
			}
		})
	}
}
