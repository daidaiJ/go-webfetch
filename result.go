package webfetch

import "time"

// FetchResult 网页抓取结果
type FetchResult struct {
	URL         string        `json:"url"`
	FinalURL    string        `json:"final_url"`
	Title       string        `json:"title"`
	StatusCode  int           `json:"status_code"`
	ContentType string        `json:"content_type"`
	Method      string        `json:"method"` // "native-http" | "headless" | "direct"
	FetchedAt   time.Time     `json:"fetched_at"`
	Elapsed     time.Duration `json:"elapsed"`

	// 输出二选一
	Mode     string `json:"mode"`              // "inline" | "saved_to_file"
	Markdown string `json:"markdown,omitempty"`
	FilePath string `json:"file_path,omitempty"`

	// saved_to_file 时的元数据
	TotalLines int    `json:"total_lines,omitempty"`
	TotalChars int    `json:"total_chars,omitempty"`
	Summary    string `json:"summary,omitempty"`
	AgentHint  string `json:"agent_hint,omitempty"`

	Error     string `json:"error,omitempty"`
	ErrorType string `json:"error_type,omitempty"`
}

// PDFResult PDF 解析结果
type PDFResult struct {
	SourcePath string `json:"source_path"`
	Title      string `json:"title"`
	PageCount  int    `json:"page_count"`
	Method     string `json:"method"` // "go-fitz"

	Mode     string `json:"mode"`
	Markdown string `json:"markdown,omitempty"`
	FilePath string `json:"file_path,omitempty"`

	TotalLines int    `json:"total_lines,omitempty"`
	TotalChars int    `json:"total_chars,omitempty"`
	Summary    string `json:"summary,omitempty"`
	AgentHint  string `json:"agent_hint,omitempty"`

	Error     string `json:"error,omitempty"`
	ErrorType string `json:"error_type,omitempty"`
}

// FetchOptions 抓取选项（FetchWithOpts 使用）
type FetchOptions struct {
	MaxInlineLines *int
	MaxInlineChars *int
}
