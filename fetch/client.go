package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Fetcher 抓取器接口
type Fetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

// ClientConfig HTTP 客户端配置
type ClientConfig struct {
	UserAgent    string
	ExtraHeaders map[string]string
	Timeout      time.Duration
	MaxRedirects int
	ProxyURL     string
}

// Client HTTP 客户端封装
type Client struct {
	httpClient *http.Client
	userAgent  string
	extraHdrs  map[string]string
}

// NewClient 创建 HTTP 客户端
func NewClient(cfg ClientConfig) (*Client, error) {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// 代理配置
	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	} else {
		// 从环境变量读取代理
		transport.Proxy = http.ProxyFromEnvironment
	}

	client := &http.Client{
		Transport:     transport,
		Timeout:       cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if cfg.MaxRedirects > 0 && len(via) >= cfg.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", cfg.MaxRedirects)
			}
			return nil
		},
	}

	return &Client{
		httpClient: client,
		userAgent:  cfg.UserAgent,
		extraHdrs:  cfg.ExtraHeaders,
	}, nil
}

// Get 发起 GET 请求
func (c *Client) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/markdown, text/html;q=0.9, text/plain;q=0.8, */*;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")

	for k, v := range c.extraHdrs {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %s: %w", rawURL, err)
	}

	return resp, nil
}

// GetBytes 下载内容为字节流（用于 PDF 等）
func (c *Client) GetBytes(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := c.Get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, rawURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return body, nil
}

// ReadBodyWithLimit 读取响应体，限制大小
func ReadBodyWithLimit(resp *http.Response, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(resp.Body, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("response body too large: %d bytes (limit %d)", len(body), maxBytes)
	}
	return body, nil
}

// IsHTMLContentType 检查是否为 HTML 内容类型
func IsHTMLContentType(ct string) bool {
	ct = strings.ToLower(strings.SplitN(ct, ";", 2)[0])
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}
