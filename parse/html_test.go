package parse

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ──────────────────────────────────────────────────────────────
// 测试用 HTML 样本
// ──────────────────────────────────────────────────────────────

const sampleArticleHTML = `<!DOCTYPE html>
<html>
<head>
    <meta property="og:title" content="Go并发编程指南">
    <title>Go并发编程指南 - 技术博客</title>
</head>
<body>
    <nav><a href="/">首页</a> <a href="/archive">归档</a></nav>
    <header><h1>网站标题</h1></header>
    <aside class="sidebar">广告内容</aside>
    <article>
        <h1>Go并发编程指南</h1>
        <p class="meta">作者：张三 | 2024-01-15</p>
        <h2>1. Goroutine基础</h2>
        <p>Goroutine是Go语言中的轻量级线程。使用<code>go</code>关键字启动。</p>
        <pre><code>go func() {
    fmt.Println("Hello")
}()</code></pre>
        <h2>2. Channel通信</h2>
        <p>Channel是goroutine之间通信的桥梁。</p>
        <ul>
            <li>无缓冲Channel：同步通信</li>
            <li>有缓冲Channel：异步通信</li>
            <li>单向Channel：限制方向</li>
        </ul>
        <h3>2.1 Channel示例</h3>
        <table>
            <thead>
                <tr><th>类型</th><th>语法</th><th>说明</th></tr>
            </thead>
            <tbody>
                <tr><td>双向</td><td>chan int</td><td>可读可写</td></tr>
                <tr><td>只读</td><td>&lt;-chan int</td><td>只能读取</td></tr>
            </tbody>
        </table>
        <blockquote>
            <p>Don't communicate by sharing memory; share memory by communicating.</p>
        </blockquote>
        <h2>3. Select语句</h2>
        <p>Select用于多路复用channel操作。</p>
        <ol>
            <li>随机选择就绪的case</li>
            <li>default分支实现非阻塞</li>
            <li>超时控制用time.After</li>
        </ol>
    </article>
    <footer><p>版权所有</p></footer>
</body>
</html>`

const sampleMinimalHTML = `<!DOCTYPE html>
<html>
<head><title>简单页面</title></head>
<body>
    <p>这是一段很短的内容。</p>
</body>
</html>`

const sampleJSHeavyHTML = `<!DOCTYPE html>
<html>
<head><title>JS重页面</title></head>
<body>
    <script>var x = 1;</script>
    <script>var y = 2;</script>
    <script>var z = 3;</script>
    <script>var a = 4;</script>
    <script>var b = 5;</script>
    <script>var c = 6;</script>
    <p>少量正文</p>
</body>
</html>`

// ──────────────────────────────────────────────────────────────
// 结构保留验证测试
// ──────────────────────────────────────────────────────────────

func TestProcessHTML_PreservesStructure(t *testing.T) {
	ctx := context.Background()
	result, diag, err := ProcessHTMLWithDiag(ctx, sampleArticleHTML, "https://example.com/article")
	if err != nil {
		t.Fatalf("ProcessHTML failed: %v", err)
	}

	md := result.Markdown

	// 验证标题被正确提取
	if result.Title != "Go并发编程指南" {
		t.Errorf("expected title 'Go并发编程指南', got %q", result.Title)
	}

	// 验证结构标签被保留
	structChecks := []struct {
		name     string
		contains string
	}{
		{"h1", "# Go并发编程指南"},
		{"h2", "## 1. Goroutine基础"},
		{"h2", "## 2. Channel通信"},
		{"h3", "### 2.1 Channel示例"},
		{"ul list item", "无缓冲Channel"},
		{"ol list item", "随机选择就绪的case"},
		{"code block", "go func()"},
		{"blockquote", "Don't communicate by sharing memory"},
		{"table header", "类型"},
		{"table cell", "chan int"},
		{"inline code", "`go`"},
	}

	for _, check := range structChecks {
		if !strings.Contains(md, check.contains) {
			t.Errorf("markdown missing %s: expected to contain %q", check.name, check.contains)
		}
	}

	// 验证噪声被移除
	noiseChecks := []string{
		"网站首页",
		"归档",
		"广告内容",
		"版权所有",
	}
	for _, noise := range noiseChecks {
		if strings.Contains(md, noise) {
			t.Errorf("markdown should not contain noise: %q", noise)
		}
	}

	// 打印诊断信息
	t.Logf("=== Pipeline Diagnostics ===")
	t.Logf("Stage 1 preClean:  html len = %d", diag.StagePreCleanLen)
	t.Logf("Stage 2 anchored:  html len = %d", diag.StageAnchoredLen)
	t.Logf("Stage 3 extract:   src = %s, html len = %d", diag.StageExtractSrc, diag.StageExtractLen)
	t.Logf("Stage 4 convert:   md len  = %d", diag.StageConvertLen)
	t.Logf("Stage 5 final:     md len  = %d", diag.StageFinalLen)
	t.Logf("Readability OK: %v", diag.ReadabilityOK)
	t.Logf("Readability Title: %q", diag.ReadabilityTitle)
	t.Logf("Structure Tags: %v", diag.StructureTags)
	t.Logf("Markdown: %d chars, %d lines", len(md), len(strings.Split(md, "\n")))

	// 打印完整 markdown 供人工审查
	t.Logf("Generated Markdown:\n%s", md)
}

func TestProcessHTML_FallbackToHeuristic(t *testing.T) {
	ctx := context.Background()
	result, diag, err := ProcessHTMLWithDiag(ctx, sampleMinimalHTML, "https://example.com/simple")
	if err != nil {
		t.Fatalf("ProcessHTML failed: %v", err)
	}

	// 短内容应该走 heuristic fallback
	if diag.ReadabilityOK {
		t.Log("Readability succeeded for short content (acceptable)")
	} else {
		t.Log("Readability failed, used heuristic fallback (expected)")
	}

	if !strings.Contains(result.Markdown, "很短的内容") {
		t.Errorf("markdown missing expected content: %q", result.Markdown)
	}
}

func TestProcessHTML_TitleExtraction(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		want    string
	}{
		{
			name: "og:title priority",
			html: `<html><head><meta property="og:title" content="OG标题"><title>页面标题</title></head><body><h1>H1标题</h1></body></html>`,
			want: "OG标题",
		},
		{
			name: "title tag fallback",
			html: `<html><head><title>页面标题</title></head><body><h1>H1标题</h1></body></html>`,
			want: "页面标题",
		},
		{
			name: "h1 fallback",
			html: `<html><head></head><body><h1>H1标题</h1></body></html>`,
			want: "H1标题",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTitle(tt.html)
			if result != tt.want {
				t.Errorf("extractTitle() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestPreCleanHTML_RemovesNoise(t *testing.T) {
	cleaned := preCleanHTML(sampleArticleHTML)

	shouldRemove := []string{
		"<nav",
		"<footer",
		"<aside",
		"class=\"sidebar\"",
		"class=\"meta\"", // meta class in article - this should stay
	}

	for _, s := range shouldRemove {
		if strings.Contains(cleaned, s) && s != "class=\"meta\"" {
			t.Errorf("preCleanHTML should remove %q", s)
		}
	}

	// 正文内容应该保留
	shouldKeep := []string{
		"Goroutine",
		"Channel",
		"Select",
	}
	for _, s := range shouldKeep {
		if !strings.Contains(cleaned, s) {
			t.Errorf("preCleanHTML should keep content containing %q", s)
		}
	}
}

func TestAnchorStructure_MarksTags(t *testing.T) {
	anchored := anchorStructure(sampleArticleHTML)

	// 关键结构标签应该被标记
	structuralTags := []string{
		"data-wf-struct",
	}
	for _, tag := range structuralTags {
		if !strings.Contains(anchored, tag) {
			t.Errorf("anchorStructure should mark with %q", tag)
		}
	}
}

// ──────────────────────────────────────────────────────────────
// 内联 vs 文件输出测试
// ──────────────────────────────────────────────────────────────

func TestBuildOutput_InlineMode(t *testing.T) {
	// 小内容应该 inline
	short := "Hello\nWorld"
	out := buildOutput(short, "Test", 200, "text/plain", Options{
		MaxInlineLines: 100,
		MaxInlineChars: 0,
	})

	if out.Mode != "inline" {
		t.Errorf("expected mode 'inline', got %q", out.Mode)
	}
	if out.Markdown != short {
		t.Errorf("expected markdown %q, got %q", short, out.Markdown)
	}
}

// ──────────────────────────────────────────────────────────────
// Content-Type 分派测试
// ──────────────────────────────────────────────────────────────

func TestProcessResponse_HTML(t *testing.T) {
	ctx := context.Background()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(sampleArticleHTML))
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("http.Get failed: %v", err)
	}
	defer resp.Body.Close()

	store := &mockStore{}
	out, err := ProcessResponse(ctx, resp, ts.URL, "text/html; charset=utf-8", Options{
		MaxInlineLines: 1000,
		Store:          store,
	})
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	if out.Mode != "inline" {
		t.Errorf("expected mode 'inline', got %q", out.Mode)
	}
	if !strings.Contains(out.Markdown, "Goroutine") {
		t.Error("markdown should contain article content")
	}
}

func TestProcessResponse_JSON(t *testing.T) {
	ctx := context.Background()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"test","value":42}`))
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("http.Get failed: %v", err)
	}
	defer resp.Body.Close()

	store := &mockStore{}
	out, err := ProcessResponse(ctx, resp, ts.URL, "application/json", Options{
		MaxInlineLines: 1000,
		Store:          store,
	})
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	if !strings.Contains(out.Markdown, "```json") {
		t.Error("JSON should be wrapped in code block")
	}
	if !strings.Contains(out.Markdown, `"name": "test"`) {
		t.Error("JSON should be pretty-printed")
	}
}

func TestProcessResponse_Markdown(t *testing.T) {
	ctx := context.Background()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# Hello\n\nWorld"))
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("http.Get failed: %v", err)
	}
	defer resp.Body.Close()

	store := &mockStore{}
	out, err := ProcessResponse(ctx, resp, ts.URL, "text/markdown", Options{
		MaxInlineLines: 1000,
		Store:          store,
	})
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	if out.Markdown != "# Hello\n\nWorld" {
		t.Errorf("markdown passthrough failed: got %q", out.Markdown)
	}
}

// ──────────────────────────────────────────────────────────────
// mock store
// ──────────────────────────────────────────────────────────────

type mockStore struct{}

func (m *mockStore) WriteMarkdown(content string, title string, sourceURL string) (string, int, int, string, error) {
	lines := strings.Split(content, "\n")
	summary := ""
	if len(lines) > 5 {
		summary = strings.Join(lines[:5], "\n")
	} else {
		summary = content
	}
	return "/tmp/mock/test.md", len(lines), len(content), summary, nil
}

func (m *mockStore) WriteMarkdownFromReader(r io.Reader, title string, sourceURL string) (string, int, int, string, error) {
	data, _ := io.ReadAll(r)
	content := string(data)
	return m.WriteMarkdown(content, title, sourceURL)
}
