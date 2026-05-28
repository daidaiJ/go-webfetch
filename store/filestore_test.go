package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileStore_WriteMarkdown(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir, time.Hour)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	content := "# Hello\n\nThis is a test.\n\n## Section\n\nMore content."
	filePath, totalLines, totalChars, summary, err := fs.WriteMarkdown(content, "Test Title", "https://example.com")
	if err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}

	// 文件应该存在
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("file should exist: %v", err)
	}

	// 文件名应该包含日期和标题 slug
	filename := filepath.Base(filePath)
	if !strings.HasSuffix(filename, ".md") {
		t.Errorf("filename should end with .md: %s", filename)
	}
	if !strings.Contains(filename, "test-title") {
		t.Errorf("filename should contain title slug: %s", filename)
	}

	// 行数和字符数应该正确
	if totalLines != 7 {
		t.Errorf("totalLines = %d, want 7", totalLines)
	}
	if totalChars != len(content) {
		t.Errorf("totalChars = %d, want %d", totalChars, len(content))
	}

	// 预览应该包含前 5 行
	if !strings.Contains(summary, "# Hello") {
		t.Errorf("summary should contain first line: %s", summary)
	}

	// 验证文件内容
	data, _ := os.ReadFile(filePath)
	if string(data) != content {
		t.Error("file content should match input")
	}
}

func TestFileStore_Clean(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	// 写入一个文件
	fs.WriteMarkdown("content", "test", "https://example.com")

	// 立即清理，不应该删除
	count, _ := fs.Clean()
	if count != 0 {
		t.Errorf("Clean() = %d, want 0 (too soon)", count)
	}

	// 等待 TTL 过期
	time.Sleep(150 * time.Millisecond)

	count, _ = fs.Clean()
	if count != 1 {
		t.Errorf("Clean() = %d, want 1 (expired)", count)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Go并发编程指南", "go并发编程指南"},
		{"  spaces  ", "spaces"},
		{"", ""},
		{"UPPER CASE", "upper-case"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
