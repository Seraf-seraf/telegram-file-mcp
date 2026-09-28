package filefetch

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/Seraf-seraf/telegram-file-mcp/internal/sendfile"
)

type resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}
type netResolver struct{}

func (netResolver) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, network, host)
}

type Client struct {
	httpClient   *http.Client
	maxFileBytes int64
	resolver     resolver
}

var nonPublicPrefixes = [...]netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("100:0:0:1::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:2::/48"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
	netip.MustParsePrefix("5f00::/16"),
}

func NewClient(httpClient *http.Client, maxFileBytes int64) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	c := &Client{maxFileBytes: maxFileBytes, resolver: netResolver{}}
	copyClient := *httpClient
	baseTransport := httpClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	if transport, ok := baseTransport.(*http.Transport); ok {
		transport = transport.Clone()
		transport.Proxy = nil
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := c.resolver.LookupNetIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, errors.New("destination resolution failed")
			}
			for _, ip := range ips {
				if !isPublic(ip) {
					return nil, errors.New("destination is not public")
				}
			}
			return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		}
		copyClient.Transport = transport
	}
	previous := httpClient.CheckRedirect
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("too many redirects")
		}
		if err := c.validateURL(req.Context(), req.URL); err != nil {
			return err
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
	c.httpClient = &copyClient
	return c
}
func isPublic(ip netip.Addr) bool {
	if ip.Is4In6() {
		ip = ip.Unmap()
	}
	if !ip.IsValid() || ip.Zone() != "" || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	if netip.MustParsePrefix("100.64.0.0/10").Contains(ip) {
		return false
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
func (c *Client) validateURL(ctx context.Context, u *url.URL) error {
	if u == nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return sendfile.NewError(sendfile.InvalidInput)
	}
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil {
		if !isPublic(ip) {
			return sendfile.NewError(sendfile.InvalidInput)
		}
		return nil
	}
	ips, err := c.resolver.LookupNetIP(ctx, "ip", u.Hostname())
	if err != nil || len(ips) == 0 {
		return sendfile.NewError(sendfile.InvalidInput)
	}
	for _, ip := range ips {
		if !isPublic(ip) {
			return sendfile.NewError(sendfile.InvalidInput)
		}
	}
	return nil
}
func (c *Client) Fetch(ctx context.Context, source sendfile.FileSource) (sendfile.File, error) {
	u, err := url.Parse(source.DownloadURL)
	if err != nil || c.validateURL(ctx, u) != nil {
		return sendfile.File{}, sendfile.NewError(sendfile.InvalidInput)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return sendfile.File{}, sendfile.NewError(sendfile.InvalidInput)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return sendfile.File{}, sendfile.NewError(sendfile.DownloadFailed)
		}
		return sendfile.File{}, sendfile.NewError(sendfile.DownloadFailed)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return sendfile.File{}, sendfile.NewError(sendfile.DownloadFailed)
	}
	if resp.ContentLength > c.maxFileBytes {
		return sendfile.File{}, sendfile.NewError(sendfile.FileTooLarge)
	}
	content, err := io.ReadAll(io.LimitReader(resp.Body, c.maxFileBytes+1))
	if err != nil {
		return sendfile.File{}, sendfile.NewError(sendfile.DownloadFailed)
	}
	if int64(len(content)) > c.maxFileBytes {
		return sendfile.File{}, sendfile.NewError(sendfile.FileTooLarge)
	}
	name := cleanName(source.FileName)
	if name == "" {
		name = cleanName(path.Base(u.Path))
	}
	if name == "" {
		name = "document"
	}
	mimeType := source.MIMEType
	if mimeType == "" {
		mimeType = resp.Header.Get("Content-Type")
	}
	return sendfile.File{Name: name, MIMEType: mimeType, SizeBytes: int64(len(content)), Content: content}, nil
}
func cleanName(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = path.Base(value)
	var b strings.Builder
	for _, r := range value {
		if !unicode.IsControl(r) && r != '/' && r != '\\' {
			b.WriteRune(r)
		}
	}
	name := strings.TrimSpace(b.String())
	if name == "." || name == ".." {
		return ""
	}
	return name
}
