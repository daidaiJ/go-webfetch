package parse

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	readability "github.com/go-shiori/go-readability"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
)

// noiseTags 需要移除的噪声标签
var noiseTags = []string{
	"script", "style", "noscript", "template",
	"header", "footer", "nav", "aside",
	"form", "input", "button", "textarea", "select",
	"svg", "iframe", "embed", "object",
	"applet", "dialog", "menu",
}

// heuristicSelectors 启发式正文选择器（按优先级）
var heuristicSelectors = []string{
	"main",
	"article",
	"[role='main']",
	".post-content",
	".article-content",
	".entry-content",
	".post-body",
	"[class*='content']",
	"[class*='article']",
	"[id*='content']",
	"[id*='article']",
	"body",
}

// HTMLResult HTML 处理结果
type HTMLResult struct {
	Title    string
	Markdown string
}

// ProcessHTML 完整的 HTML → Markdown 管线
// 管线：预清理 → 结构保护 → readability 提取 → 后处理 → html→md
func ProcessHTML(ctx context.Context, htmlContent string, baseURL string) (*HTMLResult, error) {
	// 6a. goquery 预清理：移除噪声标签（不动正文内的结构）
	cleaned := preCleanHTML(htmlContent)

	// 6a2. 结构保护：在 readability 提取前，确保关键结构标签不被误删
	// readability 可能会移除它认为"非内容"的元素，这里预先加固
	anchored := anchorStructure(cleaned)

	// 6b. go-readability 正文提取
	var contentHTML string
	var title string

	parsedURL, err := url.Parse(baseURL)
	if err == nil && parsedURL.Host != "" {
		article, rerr := readability.FromReader(strings.NewReader(anchored), parsedURL)
		if rerr == nil && len(strings.TrimSpace(article.TextContent)) >= 200 {
			contentHTML = article.Content
			title = article.Title
		}
	}

	// 6c. CSS 启发式 fallback（readability 失败或内容太短）
	if contentHTML == "" {
		contentHTML = heuristicExtract(cleaned)
	}
	if title == "" {
		title = extractTitle(htmlContent)
	}

	// 6d. 后处理：链接绝对化
	contentHTML = absolutizeLinks(contentHTML, baseURL)

	// 6e. HTML → Markdown
	converter := md.NewConverter("", true, &md.Options{EscapeMode: "disabled"})
	converter.Use(plugin.GitHubFlavored())
	registerCustomRules(converter)

	markdown, err := converter.ConvertString(contentHTML)
	if err != nil {
		markdown = extractPlainText(contentHTML)
	}

	markdown = cleanMarkdown(markdown)

	return &HTMLResult{
		Title:    title,
		Markdown: markdown,
	}, nil
}

// anchorStructure 在 readability 提取前保护关键结构信息
// readability 可能会移除它认为不重要的标签（如 article 内的 header、table 等）
// 这里通过给关键结构标签添加 data-wf-anchor 属性来标记它们
func anchorStructure(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}

	// 保护 article 内的结构标签：标题、列表、表格、代码块、引用
	structureSelectors := []string{
		"h1", "h2", "h3", "h4", "h5", "h6",
		"ul", "ol", "li",
		"table", "thead", "tbody", "tr", "th", "td",
		"pre", "code", "blockquote",
		"figure", "figcaption",
		"dl", "dt", "dd",
	}
	for _, sel := range structureSelectors {
		doc.Find(sel).Each(func(_ int, s *goquery.Selection) {
			s.SetAttr("data-wf-struct", "true")
		})
	}

	html, _ := doc.Html()
	return html
}

// registerCustomRules 注册 html-to-markdown 自定义转换规则
func registerCustomRules(converter *md.Converter) {
}

// preCleanHTML 预清理：移除噪声标签和隐藏元素
func preCleanHTML(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}

	for _, tag := range noiseTags {
		doc.Find(tag).Remove()
	}

	// 移除隐藏元素
	doc.Find("[style]").Each(func(_ int, s *goquery.Selection) {
		style, _ := s.Attr("style")
		lower := strings.ToLower(style)
		if strings.Contains(lower, "display:none") || strings.Contains(lower, "display: none") ||
			strings.Contains(lower, "visibility:hidden") || strings.Contains(lower, "visibility: hidden") {
			s.Remove()
		}
	})

	// 移除广告相关的 class/id
	doc.Find("[class]").Each(func(_ int, s *goquery.Selection) {
		cls, _ := s.Attr("class")
		lower := strings.ToLower(cls)
		if containsAny(lower, []string{"sidebar", "widget", "banner", "ad-", "-ad", "advert", "sponsor", "promo"}) {
			s.Remove()
		}
	})

	html, _ := doc.Html()
	return html
}

// heuristicExtract CSS 启发式正文提取
func heuristicExtract(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}

	for _, sel := range heuristicSelectors {
		node := doc.Find(sel)
		if node.Length() > 0 {
			html, _ := node.First().Html()
			if len(strings.TrimSpace(html)) > 200 {
				return html
			}
		}
	}

	html, _ := doc.Find("body").Html()
	if html == "" {
		return htmlContent
	}
	return html
}

// extractTitle 提取标题：og:title > <title> > <h1>
func extractTitle(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	if og, exists := doc.Find("meta[property='og:title']").Attr("content"); exists && strings.TrimSpace(og) != "" {
		return strings.TrimSpace(og)
	}

	if title := strings.TrimSpace(doc.Find("title").Text()); title != "" {
		return title
	}

	if h1 := strings.TrimSpace(doc.Find("h1").First().Text()); h1 != "" {
		return h1
	}

	return ""
}

// absolutizeLinks 将相对链接转为绝对 URL
func absolutizeLinks(htmlContent string, baseURL string) string {
	if baseURL == "" {
		return htmlContent
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}

	base, berr := url.Parse(baseURL)
	if berr != nil {
		return htmlContent
	}

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if href == "" || strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") ||
			strings.HasPrefix(href, "#") || strings.HasPrefix(href, "mailto:") {
			return
		}
		ref, rerr := url.Parse(href)
		if rerr != nil {
			return
		}
		abs := base.ResolveReference(ref)
		s.SetAttr("href", abs.String())
	})

	html, _ := doc.Html()
	return html
}

// extractPlainText 从 HTML 提取纯文本（fallback）
func extractPlainText(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}
	return strings.TrimSpace(doc.Text())
}

// cleanMarkdown 清理 Markdown 输出
func cleanMarkdown(input string) string {
	re := regexp.MustCompile(`\n{3,}`)
	input = re.ReplaceAllString(input, "\n\n")
	return strings.TrimSpace(input)
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// DiagnosticInfo 诊断信息（用于测试和调试）
type DiagnosticInfo struct {
	ReadabilityOK    bool
	ReadabilityTitle string
	ContentLength    int
	StructureTags    map[string]int // 标签名 → 数量

	// 各阶段快照
	StagePreCleanLen  int    // 预清理后 HTML 长度
	StageAnchoredLen  int    // 结构锚定后 HTML 长度
	StageExtractSrc  string // 提取来源: "readability" | "heuristic"
	StageExtractLen  int    // 提取后内容 HTML 长度
	StageConvertLen  int    // markdown 转换后长度
	StageFinalLen    int    // 最终 markdown 长度
}

// ProcessHTMLWithDiag 带诊断的 HTML 处理（用于测试）
func ProcessHTMLWithDiag(ctx context.Context, htmlContent string, baseURL string) (*HTMLResult, *DiagnosticInfo, error) {
	diag := &DiagnosticInfo{
		StructureTags: make(map[string]int),
	}

	// 1. 预清理
	cleaned := preCleanHTML(htmlContent)
	diag.StagePreCleanLen = len(cleaned)

	// 2. 结构锚定
	anchored := anchorStructure(cleaned)
	diag.StageAnchoredLen = len(anchored)

	// 统计结构标签
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(anchored))
	for _, tag := range []string{"h1", "h2", "h3", "h4", "h5", "h6", "table", "ul", "ol", "pre", "blockquote"} {
		diag.StructureTags[tag] = doc.Find(tag).Length()
	}

	// 3. 正文提取
	var contentHTML string
	var title string

	parsedURL, err := url.Parse(baseURL)
	if err == nil && parsedURL.Host != "" {
		article, rerr := readability.FromReader(strings.NewReader(anchored), parsedURL)
		if rerr == nil && len(strings.TrimSpace(article.TextContent)) >= 200 {
			contentHTML = article.Content
			title = article.Title
			diag.ReadabilityOK = true
			diag.ReadabilityTitle = title
			diag.StageExtractSrc = "readability"
		}
	}

	if contentHTML == "" {
		contentHTML = heuristicExtract(cleaned)
		diag.StageExtractSrc = "heuristic"
	}
	if title == "" {
		title = extractTitle(htmlContent)
	}
	diag.StageExtractLen = len(contentHTML)

	// 4. 链接绝对化
	contentHTML = absolutizeLinks(contentHTML, baseURL)

	// 5. HTML → Markdown
	converter := md.NewConverter("", true, &md.Options{EscapeMode: "disabled"})
	converter.Use(plugin.GitHubFlavored())
	registerCustomRules(converter)

	markdown, merr := converter.ConvertString(contentHTML)
	if merr != nil {
		markdown = extractPlainText(contentHTML)
	}
	diag.StageConvertLen = len(markdown)

	markdown = cleanMarkdown(markdown)
	diag.StageFinalLen = len(markdown)

	diag.ContentLength = len(contentHTML)

	return &HTMLResult{Title: title, Markdown: markdown}, diag, nil
}
