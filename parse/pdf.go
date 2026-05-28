package parse

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

// PDFOptions PDF 解析选项
type PDFOptions struct {
	MaxPages  int
	Heuristic bool
}

// PDFParseResult PDF 解析结果（内部）
type PDFParseResult struct {
	SourcePath string
	Title      string
	PageCount  int
	Method     string
	Markdown   string
}

// PDFParseError PDF 解析错误
type PDFParseError struct {
	Source string
	Reason string
}

func (e *PDFParseError) Error() string {
	return fmt.Sprintf("PDF parse failed (%s): %s", e.Source, e.Reason)
}

// cleanPDFText 清理 ledongthuc/pdf 输出的噪声
// 核心修复：合并被拆散的单字行（CJK 字符、数字、标点被每字一行拆开）
func cleanPDFText(raw string) string {
	// 先清理每行首尾空白
	lines := strings.Split(raw, "\n")
	var trimmed []string
	for _, line := range lines {
		trimmed = append(trimmed, strings.TrimSpace(line))
	}
	text := strings.Join(trimmed, "\n")

	// 合并紧邻的单字行（中间不能有空行）
	// "欢\n迎\n您\n加\n入" → "欢迎您加入"
	// "2023\n年\n7\n月" → "2023年7月"
	singleChar := `[\p{Han}\p{P}\d]`
	// 只匹配单行换行（\n 前后没有 \n），不跨段落
	reMerge := regexp.MustCompile(`(` + singleChar + `)\n(` + singleChar + `)`)

	// 多轮迭代：每次合并一对，直到没有变化
	for i := 0; i < 20; i++ {
		prev := text
		text = reMerge.ReplaceAllString(text, "$1$2")
		if text == prev {
			break
		}
	}

	// 第二轮：合并 "短CJK文本\n单字" 模式
	// "亲爱的\n新同事" → "亲爱的新同事"
	// "新同事\n：" → "新同事："
	reShortMerge := regexp.MustCompile(`([\p{Han}]{1,6})\n([\p{Han}\p{P}]{1,2})`)
	for i := 0; i < 10; i++ {
		prev := text
		text = reShortMerge.ReplaceAllString(text, "$1$2")
		if text == prev {
			break
		}
	}

	// 清理多余空行
	reBlank := regexp.MustCompile(`\n{3,}`)
	text = reBlank.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}

// ParsePDFFile 解析本地 PDF 文件
func ParsePDFFile(ctx context.Context, filePath string, opts PDFOptions) (*PDFParseResult, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		reason := fmt.Sprintf("open pdf: %v", err)
		if os.IsNotExist(err) {
			reason = fmt.Sprintf("file not found: %s", filePath)
		}
		return nil, &PDFParseError{Source: filePath, Reason: reason}
	}
	defer f.Close()

	pageCount := r.NumPage()
	pages := pageCount
	if opts.MaxPages > 0 && pages > opts.MaxPages {
		pages = opts.MaxPages
	}

	var allText strings.Builder
	for i := 1; i <= pages; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil || strings.TrimSpace(text) == "" {
			continue
		}
		if i > 1 {
			allText.WriteString("\n\n---\n\n")
		}
		allText.WriteString(text)
	}

	raw := allText.String()
	if raw == "" {
		return nil, &PDFParseError{
			Source: filePath,
			Reason: "no text extracted — possible causes: " +
				"1. scanned/image-based PDF (no text layer, needs OCR); " +
				"2. encrypted or password-protected PDF; " +
				"3. all pages are empty or contain only images/vector graphics",
		}
	}

	// 清理 ledongthuc/pdf 的输出噪声
	raw = cleanPDFText(raw)

	var markdown string
	if opts.Heuristic {
		markdown = heuristicStructure(raw)
	} else {
		markdown = raw
	}

	title := extractPDFTitle(filePath, markdown)

	return &PDFParseResult{
		SourcePath: filePath,
		Title:      title,
		PageCount:  pageCount,
		Method:     "ledongthuc/pdf",
		Markdown:   markdown,
	}, nil
}

// ParsePDFBytes 解析 PDF 字节流
func ParsePDFBytes(ctx context.Context, data []byte, name string, opts PDFOptions) (*PDFParseResult, error) {
	tmpFile, err := os.CreateTemp("", "webfetch-pdf-*.pdf")
	if err != nil {
		return nil, &PDFParseError{Source: name, Reason: fmt.Sprintf("create temp: %v", err)}
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return nil, &PDFParseError{Source: name, Reason: fmt.Sprintf("write temp: %v", err)}
	}
	tmpFile.Close()

	return ParsePDFFile(ctx, tmpFile.Name(), opts)
}

// extractPDFTitle 提取 PDF 标题
func extractPDFTitle(filePath string, content string) string {
	lines := strings.SplitN(content, "\n", 5)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && len(trimmed) < 200 {
			trimmed = strings.TrimLeft(trimmed, "# ")
			return trimmed
		}
	}
	return "Untitled PDF"
}

// heuristicStructure 对纯文本做启发式结构化
func heuristicStructure(raw string) string {
	lines := strings.Split(raw, "\n")
	var result strings.Builder
	inCodeBlock := false
	prevWasBlank := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if isPageNumberLine(trimmed) {
			continue
		}

		if len(line) > 0 && (len(line)-len(strings.TrimLeft(line, " ")) >= 4) {
			if !inCodeBlock {
				result.WriteString("```\n")
				inCodeBlock = true
			}
			result.WriteString(strings.TrimLeft(line, " "))
			result.WriteString("\n")
			prevWasBlank = false
			continue
		}
		if inCodeBlock {
			result.WriteString("```\n")
			inCodeBlock = false
		}

		if trimmed == "" {
			if !prevWasBlank {
				result.WriteString("\n")
				prevWasBlank = true
			}
			continue
		}
		prevWasBlank = false

		if heading, level := detectHeading(trimmed, i, lines); heading != "" {
			result.WriteString(strings.Repeat("#", level) + " " + heading + "\n\n")
			continue
		}

		if listItem := detectListItem(trimmed); listItem != "" {
			result.WriteString(listItem + "\n")
			continue
		}

		result.WriteString(trimmed + "\n\n")
	}

	if inCodeBlock {
		result.WriteString("```\n")
	}

	return strings.TrimSpace(result.String())
}

func detectHeading(line string, idx int, allLines []string) (string, int) {
	if len(line) >= 5 && isAllUpper(line) && isFollowedByBlank(idx, allLines) {
		return line, 1
	}
	if reSectionNumber.MatchString(line) && len(line) < 100 {
		return line, 1
	}
	if len(line) < 60 && isFollowedByBlank(idx, allLines) && !strings.HasSuffix(line, ".") && !strings.HasSuffix(line, "。") {
		return line, 2
	}
	return "", 0
}

var reSectionNumber = regexp.MustCompile(`^(\d+\.?\d*\.?\s|[IVXLC]+\.?\s)`)

func isAllUpper(s string) bool {
	letters := 0
	upper := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	return letters >= 3 && upper == letters
}

func isFollowedByBlank(idx int, lines []string) bool {
	if idx+1 < len(lines) {
		return strings.TrimSpace(lines[idx+1]) == ""
	}
	return true
}

func detectListItem(line string) string {
	for _, prefix := range []string{"• ", "- ", "* ", "→ ", "· "} {
		if strings.HasPrefix(line, prefix) {
			return "- " + strings.TrimPrefix(line, prefix)
		}
	}
	if reOrderedList.MatchString(line) {
		return line
	}
	return ""
}

var reOrderedList = regexp.MustCompile(`^\d+[\.\)]\s`)

func isPageNumberLine(line string) bool {
	if line == "" {
		return false
	}
	if rePageNum.MatchString(line) {
		return true
	}
	for _, r := range line {
		if !unicode.IsDigit(r) && !unicode.IsSpace(r) {
			return false
		}
	}
	return len(line) <= 6
}

var rePageNum = regexp.MustCompile(`^(page|pg\.?)\s*\d+$`)
