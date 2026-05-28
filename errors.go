package webfetch

import "fmt"

// WAFError 表示被 WAF/反爬机制拦截
type WAFError struct {
	Reason string // "cloudflare_403" | "challenge_detected" | "js_render_required"
	URL    string
}

func (e *WAFError) Error() string {
	return fmt.Sprintf("blocked by WAF (%s): %s", e.Reason, e.URL)
}

// SSRFError 表示目标 URL 解析到内网/保留 IP
type SSRFError struct {
	Host string
	IP   string
}

func (e *SSRFError) Error() string {
	return fmt.Sprintf("SSRF blocked: %s resolved to private IP %s", e.Host, e.IP)
}

// TimeoutError 表示请求超时
type TimeoutError struct {
	URL string
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("request timeout: %s", e.URL)
}

// UnsupportedContentTypeError 表示不支持的 Content-Type
type UnsupportedContentTypeError struct {
	ContentType string
}

func (e *UnsupportedContentTypeError) Error() string {
	return fmt.Sprintf("unsupported content type: %s", e.ContentType)
}

// PDFParseError 表示 PDF 解析失败
type PDFParseError struct {
	Source string // 文件路径或 "bytes:{name}"
	Reason string
}

func (e *PDFParseError) Error() string {
	return fmt.Sprintf("PDF parse failed (%s): %s", e.Source, e.Reason)
}

// EmptyContentError 表示页面返回了 HTTP 200 但内容为空或过少
// 通常是反爬机制返回的空壳页面（JS 渲染、Token 验证等）
type EmptyContentError struct {
	URL       string
	Title     string
	CharCount int
}

func (e *EmptyContentError) Error() string {
	return fmt.Sprintf("empty content from %s (title=%q, chars=%d): page may require JS rendering or is blocked by anti-bot", e.URL, e.Title, e.CharCount)
}

// NotFoundError 表示 URL 返回 404
type NotFoundError struct {
	URL        string
	StatusCode int
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("URL not found: %s (HTTP %d)", e.URL, e.StatusCode)
}
