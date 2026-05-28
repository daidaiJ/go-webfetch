package parse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// StoreWriter 文件写入接口（支持 mock）
type StoreWriter interface {
	WriteMarkdown(content string, title string, sourceURL string) (filePath string, totalLines int, totalChars int, summary string, err error)
	WriteMarkdownFromReader(r io.Reader, title string, sourceURL string) (filePath string, totalLines int, totalChars int, summary string, err error)
}

// Options 内容处理选项
type Options struct {
	MaxInlineLines   int
	MaxInlineChars   int
	MaxContentLength int // 内容超过此长度也写文件，默认 100000
	FileOutputDir    string
	Store            StoreWriter
}

// Output 内容处理输出
type Output struct {
	Mode       string // "inline" | "saved_to_file"
	Title      string
	Markdown   string // Mode=inline 时有值
	FilePath   string // Mode=saved_to_file 时有值
	TotalLines int
	TotalChars int
	Summary    string
	AgentHint  string
	StatusCode int
	ContType   string
}

// ProcessResponse 根据 Content-Type 分派处理
func ProcessResponse(ctx context.Context, resp *http.Response, baseURL string, contentType string, opts Options) (*Output, error) {
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))

	var md string
	var title string

	switch {
	case ct == "text/markdown" || ct == "text/x-markdown":
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read markdown body: %w", err)
		}
		md = string(body)
		title = titleFromURL(baseURL)
		ct = "text/markdown"

	case ct == "application/json" || strings.HasSuffix(ct, "+json"):
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read json body: %w", err)
		}
		md = formatJSON(body)
		title = titleFromURL(baseURL)
		ct = "application/json"

	case strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml"):
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read html body: %w", err)
		}
		htmlResult, err := ProcessHTML(ctx, string(body), baseURL)
		if err != nil {
			return nil, fmt.Errorf("process html: %w", err)
		}
		md = htmlResult.Markdown
		title = htmlResult.Title
		ct = "text/markdown"

	case ct == "application/pdf":
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read pdf body: %w", err)
		}
		pdfResult, err := ParsePDFBytes(ctx, body, baseURL, PDFOptions{MaxPages: 200, Heuristic: true})
		if err != nil {
			return nil, err
		}
		md = pdfResult.Markdown
		title = pdfResult.Title
		ct = "text/markdown"

	case strings.HasPrefix(ct, "text/"):
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read text body: %w", err)
		}
		md = string(body)
		title = titleFromURL(baseURL)
		ct = "text/plain"

	default:
		return nil, fmt.Errorf("unsupported content type: %s", ct)
	}

	return buildOutput(md, title, resp.StatusCode, ct, opts), nil
}

// buildOutput 决策 inline / saved_to_file
func buildOutput(markdown string, title string, statusCode int, ct string, opts Options) *Output {
	lines := strings.Split(markdown, "\n")
	totalLines := len(lines)
	totalChars := len(markdown)

	needFile := (opts.MaxInlineLines > 0 && totalLines > opts.MaxInlineLines) ||
		(opts.MaxInlineChars > 0 && totalChars > opts.MaxInlineChars) ||
		(opts.MaxContentLength > 0 && totalChars > opts.MaxContentLength)

	if !needFile {
		return &Output{
			Mode:       "inline",
			Title:      title,
			Markdown:   markdown,
			TotalLines: totalLines,
			TotalChars: totalChars,
			StatusCode: statusCode,
			ContType:   ct,
		}
	}

	// 写入文件（流式，不在内存中持有完整内容）
	filePath, fLines, fChars, summary, err := opts.Store.WriteMarkdownFromReader(
		strings.NewReader(markdown), title, "")
	if err != nil {
		// 写文件失败 fallback 到 inline
		return &Output{
			Mode:       "inline",
			Title:      title,
			Markdown:   markdown,
			TotalLines: totalLines,
			TotalChars: totalChars,
			StatusCode: statusCode,
			ContType:   ct,
		}
	}

	hint := fmt.Sprintf(
		"内容已写入 %s，共 %d 行（%d 字符），内容较长。\n"+
			"建议不要一次性读取，可用以下方式探索：\n"+
			"- read_file(path, offset=0, limit=100) 分段读取\n"+
			"- grep_search('关键字', path) 搜索定位目标段落\n"+
			"预览（前 5 行）：\n%s",
		filePath, fLines, fChars, summary,
	)

	return &Output{
		Mode:       "saved_to_file",
		Title:      title,
		FilePath:   filePath,
		TotalLines: fLines,
		TotalChars: fChars,
		Summary:    summary,
		AgentHint:  hint,
		StatusCode: statusCode,
		ContType:   ct,
	}
}

func formatJSON(body []byte) string {
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err == nil {
		if pretty, err := json.MarshalIndent(raw, "", "  "); err == nil {
			return "```json\n" + string(pretty) + "\n```"
		}
	}
	return "```json\n" + string(body) + "\n```"
}

func titleFromURL(rawURL string) string {
	parts := strings.Split(rawURL, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return "Untitled"
}
