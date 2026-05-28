# go-webfetch 设计文档 v2

> Go 原生 Web 抓取 + PDF 解析引擎 — 可复用 Go 包，集成到其他项目

---

## 1. 包定位

```
两个独立但互补的公共能力：
  ① Fetch    — 给定 URL，抓取网页内容转 Markdown
  ② ParsePDF — 给定 PDF 文件/URL，解析为结构化 Markdown

大内容统一卸载到本地 .md 文件，返回路径让 agent 自行探索
```

---

## 2. 选型决策（已确认）

| # | 决策点 | 最终选择 | 库 |
|---|--------|---------|-----|
| 1 | HTML→MD | 成熟高星维护库 | `github.com/JohannesKaufmann/html-to-markdown` (1.2k★, 活跃维护) |
| 2 | 正文提取 | go-readability + CSS 启发式 fallback | `github.com/go-shiori/go-readability` + goquery 启发式 |
| 3 | HTML 清理 | goquery（见下方说明） | `github.com/PuerkitoBio/goquery` |
| 4 | PDF 解析 | 成熟高星维护库 | `github.com/pdfcpu/pdfcpu` (1.8k★, 活跃维护, 纯 Go) |
| 5 | 无头浏览器 | 可选配置，运行时开关 | `github.com/go-rod/rod` — 最轻量 Go 无头浏览器，自动管理 Chromium |
| 6 | 文件存储 | 固定目录 + 时间哈希 | `{dir}/{YYYYMMDD_HHMMSS}_{title_slug}_{hash6}.md` |
| 7 | 包结构 | 对外表现多包，内部按需拆分 | 见 §3 |
| 8 | PDF 启发式 | 可配置开关 | `Config.PDFHeuristic bool` |

### 关于 goquery 的必要性

goquery 在有 go-readability 的情况下**仍然需要**，原因：

```
go-readability 的职责：从完整 HTML 中提取「正文区域」的 HTML 片段
goquery 的职责：  在 readability 提取前后做 DOM 操作

具体分工：
  提取前（goquery）：
    - 移除噪声标签：header/footer/nav/aside/sidebar/form/advertisement
    - 移除隐藏元素：display:none / visibility:hidden
    - 这些噪声会干扰 readability 的评分算法，提前清除提高准确率

  提取后（goquery）：
    - 链接绝对化：将相对 href 转为绝对 URL
    - 标题提取 fallback：og:title → <title> → <h1>
    - 启发式 fallback 路径：当 readability 失败时，用 goquery 做 CSS 选择器链选取

两者是「预处理 + 提取 + 后处理」的流水线关系，不是替代关系。
```

---

## 3. 包结构

```
go-webfetch/
├── go.mod                          # module github.com/yourname/go-webfetch
│
├── webfetch.go                     # 公共门面：Engine, New(), Config, Fetch(), ParsePDF*()
├── result.go                       # FetchResult, PDFResult 类型定义
├── errors.go                       # WAFError, SSRFError, TimeoutError, PDFParseError
│
├── fetch/
│   ├── client.go                   # HTTP 客户端封装（超时/UA/重定向/流式下载）
│   ├── detector.go                 # 反爬/WAF 检测
│   └── headless.go                 # //go:build headless — chromedp 封装
│
├── parse/
│   ├── html.go                     # HTML 清理 + 正文提取 + html→md 管线
│   ├── pdf.go                      # PDF 解析管线（pdfcpu + 可选启发式）
│   └── router.go                   # Content-Type 分派
│
├── guard/
│   └── ssrf.go                     # SSRF 防护：DNS→IP→内网校验
│
├── store/
│   └── filestore.go                # 文件写入 + TTL 清理 + 时间哈希命名
│
├── webfetch_test.go                # 集成测试
├── fetch/
│   └── detector_test.go            # WAF 检测单元测试
├── parse/
│   ├── html_test.go
│   └── pdf_test.go
└── testdata/
    ├── sample.html
    └── sample.pdf
```

**对外暴露：**
- `import "github.com/yourname/go-webfetch"` — 主包，包含 Engine 和所有公共类型
- 内部子包 `fetch/` `parse/` `guard/` `store/` 仅内部使用，不对外暴露
- 调用方只感知一个 import 路径

---

## 4. 公共 API

### 4.1 Config

```go
type Config struct {
    // ── HTTP ──
    UserAgent       string            // 默认 Chrome 124 UA
    ExtraHeaders    map[string]string
    Timeout         time.Duration     // 单次请求超时，默认 15s
    MaxRedirects    int               // 默认 5

    // ── 代理 ──
    ProxyURL        string            // HTTP 代理地址，如 "http://user:pass@host:port"
                                        // 支持 http/https/socks5 协议
                                        // 空字符串表示不使用代理
                                        // 也可通过 HTTP_PROXY/HTTPS_PROXY 环境变量设置

    // ── 安全 ──
    BlockPrivateIP  bool              // SSRF 防护，默认 true
    MaxURLLength    int               // 默认 2048
    MaxBodyBytes    int64             // 下载上限，默认 5MB

    // ── 内容输出 ──
    MaxInlineLines  int               // Markdown 超过此行数则写文件，默认 100
    MaxInlineChars  int               // Markdown 超过此字符数则写文件，默认 0（不启用）
                                        // 两者同时设置时，任一条件触发即写文件
    FileOutputDir   string            // 文件输出目录，默认 os.TempDir()/webfetch/
    FileTTL         time.Duration     // 文件保留时间，默认 24h

    // ── 反爬 ──
    FastFailWAF     bool              // 遇 WAF 立即失败，默认 true

    // ── 无头浏览器（可选，运行时开关）──
    Headless          bool            // 是否启用，默认 false
    HeadlessTimeout   time.Duration   // 浏览器超时，默认 30s
    HeadlessWaitAfter time.Duration   // 页面加载后等待 JS 执行，默认 500ms

    // ── PDF ──
    PDFMaxPages     int               // 最大解析页数，默认 200
    PDFHeuristic    bool              // 是否启用启发式结构化（标题/列表/段落识别），默认 true
}
```

### 4.2 Result 类型

```go
type FetchResult struct {
    URL         string        `json:"url"`
    FinalURL    string        `json:"final_url"`
    Title       string        `json:"title"`
    StatusCode  int           `json:"status_code"`
    ContentType string        `json:"content_type"`
    Method      string        `json:"method"`          // "native-http" | "headless" | "direct"
    FetchedAt   time.Time     `json:"fetched_at"`
    Elapsed     time.Duration `json:"elapsed"`

    Mode        string        `json:"mode"`            // "inline" | "saved_to_file"
    Markdown    string        `json:"markdown,omitempty"`        // Mode=inline 时有值
    FilePath    string        `json:"file_path,omitempty"`       // Mode=saved_to_file 时有值

    // saved_to_file 时的元数据 + agent 引导信息
    TotalLines  int           `json:"total_lines,omitempty"`
    TotalChars  int           `json:"total_chars,omitempty"`
    Summary     string        `json:"summary,omitempty"`         // 前 5 行预览
    AgentHint   string        `json:"agent_hint,omitempty"`      // 引导 agent 分段读取的提示

    Error       string        `json:"error,omitempty"`
    ErrorType   string        `json:"error_type,omitempty"`
}

type PDFResult struct {
    SourcePath  string        `json:"source_path"`
    Title       string        `json:"title"`
    PageCount   int           `json:"page_count"`
    Method      string        `json:"method"`           // "pdfcpu"

    Mode        string        `json:"mode"`
    Markdown    string        `json:"markdown,omitempty"`
    FilePath    string        `json:"file_path,omitempty"`

    TotalLines  int           `json:"total_lines,omitempty"`
    TotalChars  int           `json:"total_chars,omitempty"`
    Summary     string        `json:"summary,omitempty"`
    AgentHint   string        `json:"agent_hint,omitempty"`

    Error       string        `json:"error,omitempty"`
    ErrorType   string        `json:"error_type,omitempty"`
}
```

### 4.3 AgentHint 生成规则

```go
// 当 Mode="saved_to_file" 时自动生成 AgentHint
// 模板：
//   "内容已写入 {FilePath}，共 {TotalLines} 行（{TotalChars} 字符），内容较长。
//    建议不要一次性读取，可用以下方式探索：
//    - read_file(path, offset=0, limit=100) 分段读取
//    - grep_search('关键字', path) 搜索定位目标段落
//    预览（前 5 行）：
//    {Summary}"

// 示例输出：
//   "内容已写入 /tmp/webfetch/20260528_143052_design-doc_a3f2b1.md，
//    共 487 行（18230 字符），内容较长。
//    建议不要一次性读取，可用以下方式探索：
//    - read_file(path, offset=0, limit=100) 分段读取
//    - grep_search('关键字', path) 搜索定位目标段落
//    预览（前 5 行）：
//    # Design Document
//    ## Overview
//    This document describes..."
```

### 4.4 引擎 API

```go
func New(cfg Config) (*Engine, error)

// ── 网页抓取 ──
func (e *Engine) Fetch(ctx context.Context, url string) (*FetchResult, error)
func (e *Engine) FetchWithOpts(ctx context.Context, url string, opts FetchOptions) (*FetchResult, error)

// ── PDF 解析（独立工具，可单独对外暴露）──
func (e *Engine) ParsePDFFile(ctx context.Context, filePath string) (*PDFResult, error)
func (e *Engine) ParsePDFBytes(ctx context.Context, data []byte, name string) (*PDFResult, error)
func (e *Engine) ParsePDFURL(ctx context.Context, url string) (*PDFResult, error)

// ── 维护 ──
func (e *Engine) CleanFiles() (int, error)
func (e *Engine) Close() error
```

---

## 5. 处理管线

### 5.1 Fetch 管线

```
URL
 │
 ▼
┌──────────────────┐
│ 1. URL 校验       │  格式/长度/协议白名单
└───────┬──────────┘
        ▼
┌──────────────────┐
│ 2. SSRF 防护      │  DNS → IP → 内网/保留地址校验
└───────┬──────────┘
        ▼
┌──────────────────┐
│ 3. HTTP GET       │  流式下载，5MB 硬上限，手动管理重定向
└───────┬──────────┘
        ▼
┌──────────────────┐
│ 4. WAF 检测       │  状态码/响应头/响应体前8KB 扫描
└───────┬──────────┘
        │
   ┌────┴──────────────┐
   │ 触发 WAF?          │
   ├─ 是 + Headless=on  → chromedp 渲染 → 取渲染后 DOM → 走 5
   ├─ 是 + Headless=off → 返回 WAFError（快速失败，不重试）
   └─ 否 ──────────────▼
┌──────────────────┐
│ 5. Content-Type   │
│    分派            │
└───────┬──────────┘
        │
   ┌────┼──────────┬──────────┬──────────┐
   ▼    ▼          ▼          ▼          ▼
  HTML  text/md   PDF        JSON    text/*
   │    │          │          │        │
   ▼    ▼          ▼          ▼        ▼
  ⑥    直通      调用内部    JSON     原样
              ParsePDF管线  格式化    返回
   │               │
   └───────┬───────┘
           ▼
┌──────────────────┐
│ 7. 输出决策       │  lines > MaxInlineLines || chars > MaxInlineChars ?
└───────┬──────────┘        （任一条件满足即写文件，MaxInlineChars=0 则不参与判断）
   ┌────┴────┐
   ▼         ▼
  inline    saved_to_file
  返回文本   写入文件，返回路径+行数+AgentHint
```

### 5.2 HTML 处理子管线（步骤⑥详细展开）

```
原始 HTML
 │
 ▼
┌─────────────────────────────────┐
│ ⑥-a goquery 预清理              │
│  移除噪声标签：                   │
│    header, footer, nav, aside,   │
│    .sidebar, .ad, .advertisement,│
│    form, input, button, svg,     │
│    iframe, embed, object         │
│  移除隐藏元素：                   │
│    [style*="display:none"]       │
│    [style*="visibility:hidden"]  │
└───────┬─────────────────────────┘
        ▼
┌─────────────────────────────────┐
│ ⑥-b go-readability 正文提取      │
│  输入：预清理后的完整 HTML         │
│  输出：正文区域的 HTML 片段        │
│  附带：标题、作者、发布日期        │
└───────┬─────────────────────────┘
        │
   ┌────┴──────────────┐
   │ 提取成功?           │
   │ (正文 ≥ 200 字符)   │
   ├─ 是 → 使用 readability 结果
   └─ 否 → ⑥-c CSS 启发式 fallback
              main > article > [role=main]
              > .post-content > .article-content
              > [class*="content"] > [id*="content"]
              > body
        │
        ▼
┌─────────────────────────────────┐
│ ⑥-d goquery 后处理               │
│  链接绝对化：相对 href → 绝对 URL  │
│  标题提取：og:title > <title> > h1│
└───────┬─────────────────────────┘
        ▼
┌─────────────────────────────────┐
│ ⑥-e html-to-markdown 转换        │
│  heading_style: ATX              │
│  strip: [img] (可选保留)          │
│  保留：表格、代码块、链接、列表    │
└───────┬─────────────────────────┘
        ▼
     Markdown 文本
```

### 5.3 ParsePDF 管线

```
输入：文件路径 / []byte / URL
 │
 ▼
┌──────────────────┐
│ 1. 输入归一化      │  URL → 下载到临时文件；bytes → 写临时文件
└───────┬──────────┘
        ▼
┌──────────────────┐
│ 2. pdfcpu 逐页提取 │  提取每页纯文本 + 元数据（标题/作者）
└───────┬──────────┘
        ▼
┌──────────────────┐
│ 3. 是否启用启发式? │  Config.PDFHeuristic
└───────┬──────────┘
        │
   ┌────┴────┐
   │         │
   ▼ off     ▼ on
 直接拼接    ┌──────────────────────────┐
 页间 ---    │ 3-a 段落合并              │
             │   单行断句 → 合并为段落    │
             │                           │
             │ 3-b 标题识别               │
             │   • 全大写行 → # 标题      │
             │   • 短行(<60char)+后跟空行  │
             │     → ## 标题              │
             │   • 数字编号开头(1./2./I.)  │
             │     + 短行 → # 标题        │
             │                           │
             │ 3-c 列表识别               │
             │   • 行首 •/-/*/→ → 无序列表 │
             │   • 行首 数字. → 有序列表   │
             │                           │
             │ 3-d 代码块识别              │
             │   • 连续缩进 ≥4 空格行     │
             │     → ``` 包裹             │
             │                           │
             │ 3-e 清理                   │
             │   • 移除纯页码行            │
             │   • 移除页眉页脚重复行      │
             │   • 页间插入 --- 分隔线     │
             └──────────────────────────┘
        │
        ▼
┌──────────────────┐
│ 4. 拼装 Markdown   │  标题 + 元数据头 + 正文
└───────┬──────────┘
        ▼
┌──────────────────┐
│ 5. 输出决策        │  同 Fetch：lines > MaxInlineLines || chars > MaxInlineChars ?
└──────────────────┘
```

**启发式可配置的意义：**

```go
// 场景 A：学术论文/技术文档 → 启用启发式（结构信息有价值）
engine, _ := webfetch.New(webfetch.Config{PDFHeuristic: true})

// 场景 B：纯文本型 PDF（发票/表格/表单）→ 关闭启发式（避免误判）
engine, _ := pdfcpu.New(pdfcpu.Config{PDFHeuristic: false})
```

启发式基于 pdfcpu 提取的**纯文本**做模式匹配，不依赖 HTML 信息。
pdfcpu 本身已提供文本+位置信息，足够支撑标题/列表/段落的启发式识别。

---

## 6. 文件命名策略

```
{FileOutputDir}/{YYYYMMDD}_{HHMMSS}_{title_slug}_{hash6}.md

示例：
  /tmp/webfetch/20260528_143052_go-webfetch-design_a3f2b1.md
  /tmp/webfetch/20260528_143200_annual-report-2025_f8c912.md

组成：
  YYYYMMDD_HHMMSS  — 抓取时间，agent 一眼能看出文件新旧
  title_slug       — 标题的 URL-safe slug（截断到 40 字符，仅保留 [a-z0-9-]）
  hash6            — sha256(url)[:6]，防同名冲突

agent 友好性：
  - 文件名自带时间和标题含义，不需要打开文件就能判断内容
  - 相同 URL 不同时间抓取会产生不同文件名（时间戳区分）
  - 固定目录，agent 可以 ls/glob 列出所有缓存文件
```

---

## 7. 反爬 WAF 检测

```go
// 检测维度（全部基于单次响应，< 5ms 开销）

// 1. HTTP 状态码 + 响应头
403/429 + Server: cloudflare    → waf_blocked:cloudflare
403/429 + CF-RAY 头存在         → waf_blocked:cloudflare
403/429 + x-datadome-* 头      → waf_blocked:datadome

// 2. 响应体前 8KB 关键词扫描
challenge-platform, turnstile, jschl, cf-chl-bypass,
checking your browser, just a moment, 安全验证, 验证码, 人机验证

// 3. 已知反爬域名（精确+后缀匹配）
zhihu.com, weixin.qq.com, x.com, twitter.com, reddit.com

// 4. JS 渲染依赖检测
正文 < 500 字符 且 <script> 标签 > 5 个 → js_render_required

// 快速失败策略：
//   检测到 WAF + Headless=off → 立即返回 WAFError
//   检测到 WAF + Headless=on  → 降级到 chromedp
//   任何情况下不重试原始请求
```

---

## 8. 错误体系

```go
// 调用方可 errors.As 判断错误类型

type WAFError struct {
    Reason string   // "cloudflare_403" | "challenge_detected" | "js_render_required"
    URL    string
}
func (e *WAFError) Error() string

type SSRFError struct {
    Host string
    IP   string
}

type TimeoutError struct {
    URL string
}

type UnsupportedContentTypeError struct {
    ContentType string
}

type PDFParseError struct {
    Source string   // 文件路径或 "bytes:{name}"
    Reason string
}
```

---

## 9. 无头浏览器集成细节

```
编译标签控制：
  go build             → 不编译 headless.go，Engine.Headless=true 时 New() 返回错误
  go build -tags headless → 编译 chromedp 封装，Headless=true 正常工作

运行时行为：
  Config.Headless = false（默认）→ 纯 HTTP 抓取，遇到 WAF 快速失败
  Config.Headless = true          → 遇到 WAF 时自动降级到 chromedp
                                    渲染完成后取 innerHTML 走正常 HTML 管线

资源管理：
  Engine.Close() 负责关闭 chromedp allocator context
  单个 Engine 实例复用浏览器实例，不每次启动/关闭
  HeadlessTimeout 控制单次导航超时（默认 30s）
```

---

## 10. 使用示例

```go
// ── 基本用法（默认按行数判断）──
engine, _ := webfetch.New(webfetch.Config{
    MaxInlineLines: 100,
    FileOutputDir:  "/tmp/webfetch-cache",
    PDFHeuristic:   true,
})

// ── 也可以按字符数判断，或两者同时设置 ──
engine, _ := webfetch.New(webfetch.Config{
    MaxInlineLines: 200,             // 超过 200 行写文件
    MaxInlineChars: 50_000,          // 或超过 50000 字符写文件（任一触发）
    FileOutputDir:  "/tmp/webfetch-cache",
})

// ── 只按字符数，不按行数 ──
engine, _ := webfetch.New(webfetch.Config{
    MaxInlineLines: 0,               // 0 = 不启用行数判断
    MaxInlineChars: 30_000,
    FileOutputDir:  "/tmp/webfetch-cache",
})

// 网页抓取
r, _ := engine.Fetch(ctx, "https://example.com/article")
switch r.Mode {
case "inline":
    fmt.Println(r.Markdown)
case "saved_to_file":
    fmt.Printf("内容已保存: %s (%d 行)\n", r.FilePath, r.TotalLines)
    fmt.Printf("预览: %s\n", r.Summary)
    // → agent 用 read_file(r.FilePath, offset=0, limit=100) 逐段读取
}

// PDF 解析
p, _ := engine.ParsePDFFile(ctx, "/path/to/report.pdf")
fmt.Printf("PDF: %d 页 → %s (%d 行)\n", p.PageCount, p.FilePath, p.TotalLines)

// 远程 PDF（自动下载+解析）
p, _ = engine.ParsePDFURL(ctx, "https://example.com/whitepaper.pdf")
```
