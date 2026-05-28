package webfetch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/daidaiJ/go-webfetch/fetch"
	"github.com/daidaiJ/go-webfetch/guard"
	"github.com/daidaiJ/go-webfetch/parse"
	"github.com/daidaiJ/go-webfetch/store"
)

// Config 引擎配置
type Config struct {
	// ── HTTP ──
	UserAgent    string            // 默认 Chrome 124 UA
	ExtraHeaders map[string]string // 额外请求头
	Timeout      time.Duration     // 单次请求超时，默认 15s
	MaxRedirects int               // 最大重定向次数，默认 5

	// ── 代理 ──
	ProxyURL string // HTTP 代理地址，如 "http://user:pass@host:port"
	// 支持 http/https/socks5 协议
	// 空字符串表示不使用代理
	// 也可通过 HTTP_PROXY/HTTPS_PROXY 环境变量设置

	// ── 安全 ──
	BlockPrivateIP bool  // SSRF 防护，默认 true
	MaxURLLength   int   // 默认 2048
	MaxBodyBytes   int64 // 下载上限，默认 5MB

	// ── 内容输出 ──
	MaxInlineLines   int           // Markdown 超过此行数则写文件，默认 100
	MaxInlineChars   int           // Markdown 超过此字符数则写文件，默认 0（不启用）
	MaxContentLength int           // 输出给 LLM 的内容上限，默认 100000（100KB）
	FileOutputDir    string        // 文件输出目录，默认 os.TempDir()/webfetch/
	FileTTL          time.Duration // 文件保留时间，默认 24h

	// ── 反爬 ──
	FastFailWAF bool // 遇 WAF 立即失败，默认 true

	// ── PDF ──
	PDFMaxPages  int  // 最大解析页数，默认 200
	PDFHeuristic bool // 是否启用启发式结构化，默认 true
}

func (c Config) withDefaults() Config {
	if c.UserAgent == "" {
		c.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	}
	if c.Timeout == 0 {
		c.Timeout = 15 * time.Second
	}
	if c.MaxRedirects == 0 {
		c.MaxRedirects = 5
	}
	if c.MaxURLLength == 0 {
		c.MaxURLLength = 2048
	}
	if c.MaxBodyBytes == 0 {
		c.MaxBodyBytes = 5 * 1024 * 1024
	}
	if c.MaxInlineLines == 0 {
		c.MaxInlineLines = 100
	}
	if c.MaxContentLength == 0 {
		c.MaxContentLength = 100000
	}
	if c.FileOutputDir == "" {
		c.FileOutputDir = filepath.Join(os.TempDir(), "webfetch")
	}
	if c.FileTTL == 0 {
		c.FileTTL = 24 * time.Hour
	}
	if c.PDFMaxPages == 0 {
		c.PDFMaxPages = 200
	}
	c.FastFailWAF = true // 始终快速失败
	c.PDFHeuristic = true
	return c
}

// Engine 抓取引擎
type Engine struct {
	config   Config
	client   *fetch.Client
	detector *fetch.Detector
	store    *store.FileStore
}

// New 创建引擎实例
func New(cfg Config) (*Engine, error) {
	cfg = cfg.withDefaults()

	client, err := fetch.NewClient(fetch.ClientConfig{
		UserAgent:    cfg.UserAgent,
		ExtraHeaders: cfg.ExtraHeaders,
		Timeout:      cfg.Timeout,
		MaxRedirects: cfg.MaxRedirects,
		ProxyURL:     cfg.ProxyURL,
	})
	if err != nil {
		return nil, fmt.Errorf("create http client: %w", err)
	}

	fs, err := store.NewFileStore(cfg.FileOutputDir, cfg.FileTTL)
	if err != nil {
		return nil, fmt.Errorf("create file store: %w", err)
	}

	e := &Engine{
		config:   cfg,
		client:   client,
		detector: fetch.NewDetector(),
		store:    fs,
	}

	return e, nil
}

// Fetch 抓取 URL 并返回结果
func (e *Engine) Fetch(ctx context.Context, url string) (*FetchResult, error) {
	return e.fetchInternal(ctx, url, nil)
}

// FetchWithOpts 带选项的抓取
func (e *Engine) FetchWithOpts(ctx context.Context, url string, opts FetchOptions) (*FetchResult, error) {
	return e.fetchInternal(ctx, url, &opts)
}

// ParsePDFFile 解析本地 PDF 文件
func (e *Engine) ParsePDFFile(ctx context.Context, filePath string) (*PDFResult, error) {
	pr, err := parse.ParsePDFFile(ctx, filePath, parse.PDFOptions{
		MaxPages:  e.config.PDFMaxPages,
		Heuristic: e.config.PDFHeuristic,
	})
	if err != nil {
		return nil, err
	}
	return pdfParseResultToPDFResult(pr), nil
}

// ParsePDFBytes 解析 PDF 字节流
func (e *Engine) ParsePDFBytes(ctx context.Context, data []byte, name string) (*PDFResult, error) {
	pr, err := parse.ParsePDFBytes(ctx, data, name, parse.PDFOptions{
		MaxPages:  e.config.PDFMaxPages,
		Heuristic: e.config.PDFHeuristic,
	})
	if err != nil {
		return nil, err
	}
	return pdfParseResultToPDFResult(pr), nil
}

// ParsePDFURL 下载并解析远程 PDF
func (e *Engine) ParsePDFURL(ctx context.Context, url string) (*PDFResult, error) {
	body, err := e.client.GetBytes(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("download PDF: %w", err)
	}
	pr, err := parse.ParsePDFBytes(ctx, body, url, parse.PDFOptions{
		MaxPages:  e.config.PDFMaxPages,
		Heuristic: e.config.PDFHeuristic,
	})
	if err != nil {
		return nil, err
	}
	return pdfParseResultToPDFResult(pr), nil
}

func pdfParseResultToPDFResult(pr *parse.PDFParseResult) *PDFResult {
	return &PDFResult{
		SourcePath: pr.SourcePath,
		Title:      pr.Title,
		PageCount:  pr.PageCount,
		Method:     pr.Method,
		Markdown:   pr.Markdown,
	}
}

// CleanFiles 清理过期的缓存文件
func (e *Engine) CleanFiles() (int, error) {
	return e.store.Clean()
}

// Close 释放资源
func (e *Engine) Close() error {
	return nil
}

// fetchInternal 内部抓取实现
func (e *Engine) fetchInternal(ctx context.Context, rawURL string, opts *FetchOptions) (*FetchResult, error) {
	start := time.Now()

	// 1. URL 校验
	rawURL = normalizeURL(rawURL)
	if err := guard.ValidateURL(rawURL, e.config.MaxURLLength); err != nil {
		return nil, err
	}

	// 2. SSRF 防护
	if e.config.BlockPrivateIP {
		if err := guard.CheckPrivateIP(ctx, rawURL); err != nil {
			return nil, err
		}
	}

	maxLines := e.config.MaxInlineLines
	maxChars := e.config.MaxInlineChars
	if opts != nil {
		if opts.MaxInlineLines != nil {
			maxLines = *opts.MaxInlineLines
		}
		if opts.MaxInlineChars != nil {
			maxChars = *opts.MaxInlineChars
		}
	}
	parseOpts := parse.Options{
		MaxInlineLines:   maxLines,
		MaxInlineChars:   maxChars,
		MaxContentLength: e.config.MaxContentLength,
		Store:            e.store,
	}

	// 3. 第一次尝试
	out, finalURL, err := e.fetchOnce(ctx, rawURL, parseOpts)
	if err != nil {
		return nil, err
	}
	method := "native-http"

	// 4. 空内容检测
	if out.Title == "" && out.TotalChars < 50 && len(strings.TrimSpace(out.Markdown)) < 50 {
		return nil, &EmptyContentError{URL: rawURL, Title: out.Title, CharCount: out.TotalChars}
	}

	// 5. 组装 FetchResult
	return &FetchResult{
		URL:         rawURL,
		FinalURL:    finalURL,
		Title:       out.Title,
		StatusCode:  out.StatusCode,
		ContentType: out.ContType,
		Mode:        out.Mode,
		Markdown:    out.Markdown,
		FilePath:    out.FilePath,
		TotalLines:  out.TotalLines,
		TotalChars:  out.TotalChars,
		Summary:     out.Summary,
		AgentHint:   out.AgentHint,
		Method:      method,
		FetchedAt:   time.Now(),
		Elapsed:     time.Since(start),
	}, nil
}

// fetchOnce 单次 HTTP 抓取 + 处理
func (e *Engine) fetchOnce(ctx context.Context, rawURL string, opts parse.Options) (*parse.Output, string, error) {
	resp, err := e.client.Get(ctx, rawURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	// 4xx/5xx 快速失败
	if resp.StatusCode >= 400 {
		if resp.StatusCode == 404 || resp.StatusCode == 410 {
			return nil, "", &NotFoundError{URL: rawURL, StatusCode: resp.StatusCode}
		}
		return nil, "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, rawURL)
	}

	// WAF 检测
	detected, reason := e.detector.Detect(resp, resp.Body)
	if detected {
		return nil, "", &WAFError{Reason: reason, URL: rawURL}
	}

	contentType := resp.Header.Get("Content-Type")
	out, err := parse.ProcessResponse(ctx, resp, rawURL, contentType, opts)
	if err != nil {
		return nil, "", err
	}
	return out, resp.Request.URL.String(), nil
}

// normalizeURL 对 URL 做规范化处理
func normalizeURL(rawURL string) string {
	// GitHub blob URL → raw URL
	// https://github.com/user/repo/blob/branch/path → https://raw.githubusercontent.com/user/repo/branch/path
	if strings.Contains(rawURL, "github.com") && strings.Contains(rawURL, "/blob/") {
		rawURL = strings.Replace(rawURL, "github.com", "raw.githubusercontent.com", 1)
		rawURL = strings.Replace(rawURL, "/blob/", "/", 1)
	}
	return rawURL
}
