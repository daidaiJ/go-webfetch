package store

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// FileStore 文件存储管理
type FileStore struct {
	dir string
	ttl time.Duration
}

// NewFileStore 创建文件存储
func NewFileStore(dir string, ttl time.Duration) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	return &FileStore{dir: dir, ttl: ttl}, nil
}

// WriteMarkdown 写入 Markdown 文件，返回文件路径
func (fs *FileStore) WriteMarkdown(content string, title string, sourceURL string) (filePath string, totalLines int, totalChars int, summary string, err error) {
	lines := strings.Split(content, "\n")
	totalLines = len(lines)
	totalChars = len(content)

	// 生成预览（前 5 行有效内容，跳过空行和分隔线）
	summary = strings.Join(extractPreview(lines, 5), "\n")

	// 生成文件名
	filename := fs.buildFilename(title, sourceURL)
	filePath = filepath.Join(fs.dir, filename)

	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		return "", 0, 0, "", fmt.Errorf("write file: %w", err)
	}

	return filePath, totalLines, totalChars, summary, nil
}

// WriteMarkdownFromReader 流式写入，不在内存中持有完整内容
func (fs *FileStore) WriteMarkdownFromReader(r io.Reader, title string, sourceURL string) (filePath string, totalLines int, totalChars int, summary string, err error) {
	filename := fs.buildFilename(title, sourceURL)
	filePath = filepath.Join(fs.dir, filename)

	f, err := os.Create(filePath)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	defer writer.Flush()

	scanner := bufio.NewScanner(r)
	var previewLines []string
	lineCount := 0
	charCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		charCount += len(line) + 1 // +1 for newline
		if len(previewLines) < 5 && !isBlankOrSeparator(line) {
			previewLines = append(previewLines, line)
		}
		writer.WriteString(line)
		writer.WriteByte('\n')
	}

	if err := scanner.Err(); err != nil {
		return "", 0, 0, "", fmt.Errorf("read content: %w", err)
	}

	summary = strings.Join(previewLines, "\n")
	return filePath, lineCount, charCount, summary, nil
}

// buildFilename 生成文件名：{YYYYMMDD}_{HHMMSS}_{title_slug}_{hash6}.md
func (fs *FileStore) buildFilename(title string, sourceURL string) string {
	now := time.Now()
	datePart := now.Format("20060102_150405")

	slug := slugify(title)
	if slug == "" {
		slug = "untitled"
	}
	// 截断到 40 字符
	if len(slug) > 40 {
		slug = slug[:40]
		slug = strings.TrimRight(slug, "-_")
	}

	hash := shortHash(sourceURL)

	return fmt.Sprintf("%s_%s_%s.md", datePart, slug, hash)
}

// Clean 清理过期文件，返回清理数量
func (fs *FileStore) Clean() (int, error) {
	entries, err := os.ReadDir(fs.dir)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > fs.ttl {
			if err := os.Remove(filepath.Join(fs.dir, entry.Name())); err == nil {
				count++
			}
		}
	}
	return count, nil
}

func shortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:3])[:6]
}

var slugRe = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// separatorRe 匹配 Markdown 分隔线：---、***、___ 及其空格变体
var separatorRe = regexp.MustCompile(`^[\s]*([-*_]\s*){3,}[\s]*$`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// isBlankOrSeparator 判断行是否为空行或 Markdown 分隔线
func isBlankOrSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	return separatorRe.MatchString(trimmed)
}

// extractPreview 从行切片中提取前 n 行有意义的内容（跳过空行和分隔线）
func extractPreview(lines []string, n int) []string {
	var result []string
	for _, line := range lines {
		if isBlankOrSeparator(line) {
			continue
		}
		result = append(result, line)
		if len(result) >= n {
			break
		}
	}
	return result
}
