package webfetch

import (
	"context"
	"os"
	"strings"
	"testing"
)

// logFirstLines 打印 markdown 开头 N 行，用于快速验证抓取质量
func logFirstLines(t *testing.T, md string, n int) {
	t.Helper()
	lines := strings.Split(md, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	t.Logf("── Content preview (%d lines) ──", len(lines))
	for i, line := range lines {
		t.Logf("  %2d| %s", i+1, line)
	}
	t.Logf("── end ──")
}

func TestFetch_Integration_CNblogs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine, err := New(Config{
		MaxInlineLines: 50, // 故意设小，触发文件写入
		MaxInlineChars: 0,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	result, err := engine.Fetch(ctx, "https://www.cnblogs.com/OpenCSG/p/19672242")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// 基本字段验证
	if result.URL == "" {
		t.Error("URL should not be empty")
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	if result.Method != "native-http" {
		t.Errorf("Method = %q, want 'native-http'", result.Method)
	}

	t.Logf("Title: %q", result.Title)
	t.Logf("Mode: %s | Status: %d | Lines: %d, Chars: %d | Elapsed: %v",
		result.Mode, result.StatusCode, result.TotalLines, result.TotalChars, result.Elapsed)

	if result.Mode == "saved_to_file" {
		if result.TotalChars < 50 {
			t.Errorf("TotalChars = %d, want >= 50", result.TotalChars)
		}
		logFirstLines(t, result.Summary, 5)
	} else {
		trimmed := strings.TrimSpace(result.Markdown)
		if len(trimmed) < 50 {
			t.Errorf("content too short (%d chars)", len(trimmed))
		}
		logFirstLines(t, result.Markdown, 10)
	}
}

func TestFetch_Integration_ShortPage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine, err := New(Config{
		MaxInlineLines: 1000,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	result, err := engine.Fetch(ctx, "https://httpbin.org/html")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if result.Mode != "inline" {
		t.Errorf("short page should be inline, got %s", result.Mode)
	}

	if !strings.Contains(result.Markdown, "Herman Melville") {
		t.Error("httpbin html should contain 'Herman Melville'")
	}
}

func TestFetch_WAFBlocked(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	_, err = engine.Fetch(ctx, "https://www.zhihu.com/question/123456")

	if err == nil {
		t.Skip("zhihu might not block in this environment")
	}

	if wafErr, ok := err.(*WAFError); ok {
		t.Logf("Correctly detected WAF: %s", wafErr.Reason)
	} else {
		t.Logf("Non-WAF error (acceptable): %v", err)
	}
}

func TestFetch_Integration_V2EX(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine, err := New(Config{MaxInlineLines: 200})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	result, err := engine.Fetch(ctx, "https://global.v2ex.co/t/1216076")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}

	t.Logf("Title: %q", result.Title)
	t.Logf("Mode: %s | Lines: %d, Chars: %d | Elapsed: %v", result.Mode, result.TotalLines, result.TotalChars, result.Elapsed)

	if result.Mode == "saved_to_file" {
		if result.TotalChars < 50 {
			t.Errorf("TotalChars = %d, want >= 50", result.TotalChars)
		}
		logFirstLines(t, result.Summary, 5)
	} else {
		trimmed := strings.TrimSpace(result.Markdown)
		if len(trimmed) < 50 {
			t.Errorf("content too short (%d chars)", len(trimmed))
		}
		logFirstLines(t, result.Markdown, 10)
	}
}

func TestFetch_Integration_PkgGoDev(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine, err := New(Config{MaxInlineLines: 200})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	result, err := engine.Fetch(ctx, "https://pkg.go.dev/github.com/pdfcpu/pdfcpu")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}

	t.Logf("Title: %q", result.Title)
	t.Logf("Mode: %s | Lines: %d, Chars: %d | Elapsed: %v", result.Mode, result.TotalLines, result.TotalChars, result.Elapsed)

	if result.Mode == "saved_to_file" {
		if result.TotalChars < 50 {
			t.Errorf("TotalChars = %d, want >= 50", result.TotalChars)
		}
		logFirstLines(t, result.Summary, 5)
	} else {
		trimmed := strings.TrimSpace(result.Markdown)
		if len(trimmed) < 50 {
			t.Errorf("content too short (%d chars)", len(trimmed))
		}
		logFirstLines(t, result.Markdown, 10)
	}
}

func TestFetch_Integration_SegmentFault_WAF(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	result, err := engine.Fetch(ctx, "https://segmentfault.com/q/1010000047798379")

	if err != nil {
		t.Logf("Fetch error: %v", err)
		if emptyErr, ok := err.(*EmptyContentError); ok {
			t.Logf("Correctly detected empty content: %s", emptyErr)
		} else if wafErr, ok := err.(*WAFError); ok {
			t.Logf("WAF detected and not bypassed: reason=%s", wafErr.Reason)
		}
		return
	}

	t.Logf("Title: %q", result.Title)
	t.Logf("Mode: %s | Method: %s | Lines: %d, Chars: %d | Elapsed: %v",
		result.Mode, result.Method, result.TotalLines, result.TotalChars, result.Elapsed)
	logFirstLines(t, result.Markdown, 10)
}

func TestFetch_Integration_JieMian(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine, err := New(Config{MaxInlineLines: 200})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	result, err := engine.Fetch(ctx, "https://www.jiemian.com/article/14494514.html")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	t.Logf("Title: %q", result.Title)
	t.Logf("Mode: %s | Method: %s | Lines: %d, Chars: %d | Elapsed: %v",
		result.Mode, result.Method, result.TotalLines, result.TotalChars, result.Elapsed)

	if result.Mode == "saved_to_file" {
		logFirstLines(t, result.Summary, 5)
	} else {
		logFirstLines(t, result.Markdown, 10)
	}
}

func TestFetch_Integration_36Kr(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine, err := New(Config{MaxInlineLines: 200})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	result, err := engine.Fetch(ctx, "https://www.36kr.com/p/1726927246851333")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	t.Logf("Title: %q", result.Title)
	t.Logf("Mode: %s | Method: %s | Lines: %d, Chars: %d | Elapsed: %v",
		result.Mode, result.Method, result.TotalLines, result.TotalChars, result.Elapsed)

	if result.Mode == "saved_to_file" {
		logFirstLines(t, result.Summary, 5)
	} else {
		logFirstLines(t, result.Markdown, 10)
	}
}

func TestParsePDFFile(t *testing.T) {
	pdfPath := os.Getenv("TEST_PDF_PATH")
	if pdfPath == "" {
		t.Skip("set TEST_PDF_PATH to test PDF parsing")
	}

	engine, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	result, err := engine.ParsePDFFile(context.Background(), pdfPath)
	if err != nil {
		t.Fatalf("ParsePDFFile: %v", err)
	}

	t.Logf("Title: %q", result.Title)
	t.Logf("Pages: %d | Method: %s | Chars: %d", result.PageCount, result.Method, len(result.Markdown))
	logFirstLines(t, result.Markdown, 10)
}

func TestFetch_SSRFBlocked(t *testing.T) {
	engine, err := New(Config{
		BlockPrivateIP: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	_, err = engine.Fetch(ctx, "http://127.0.0.1:8080/admin")

	if err == nil {
		t.Error("should block localhost access")
	}
	t.Logf("Correctly blocked: %v", err)
}
