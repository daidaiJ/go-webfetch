# go-webfetch

Go 语言实现的网页抓取与 PDF 解析库，面向 AI Agent 场景设计。

## 核心亮点

### 1. 智能 HTML → Markdown 转换

不是简单的标签剥离。管线式处理：预清理噪声标签 → 结构保护 → readability 正文提取 → 启发式 fallback → GFM Markdown 转换（含表格、代码块、列表）。

```
原始 HTML → 去噪 → 锚定结构 → readability 提取 → html-to-markdown(GFM) → 干净 Markdown
```

### 2. 安全防护

- **SSRF 防护**：自动拦截私有 IP / 内网地址请求
- **WAF 检测**：识别 Cloudflare、SafeLine（长亭雷池）、DataDome 等常见 WAF 拦截
- **404 快速失败**：明确的 `NotFoundError`，方便 Agent 快速判断 URL 是否有效
- **空内容检测**：HTTP 200 但内容为空时返回 `EmptyContentError`（常见于反爬空壳页面）

### 3. PDF 解析

支持本地文件和远程 URL 的 PDF 解析，自动清理中文 PDF 的单字换行噪声。

### 4. Agent 友好输出

- 大内容自动写入文件，返回文件路径 + 预览摘要
- AgentHint 提示 Agent 如何分段读取（`read_file` + `grep_search`）
- 流式写入，不在内存中持有完整大文件
- 100KB 内容上限，避免撑爆 LLM 上下文

### 5. GitHub URL 自动转换

`github.com/user/repo/blob/branch/path` 自动转为 `raw.githubusercontent.com/...`，Agent 传入 blob URL 也能正常抓取。

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    webfetch "go-webfetch"
)

func main() {
    engine, err := webfetch.New(webfetch.Config{})
    if err != nil {
        panic(err)
    }
    defer engine.Close()

    result, err := engine.Fetch(context.Background(), "https://example.com")
    if err != nil {
        // err 可能是 *WAFError, *NotFoundError, *EmptyContentError, *SSRFError
        panic(err)
    }

    fmt.Printf("Title: %s\n", result.Title)
    fmt.Printf("Mode:  %s\n", result.Mode) // "inline" 或 "saved_to_file"
    if result.Mode == "inline" {
        fmt.Println(result.Markdown)
    } else {
        fmt.Printf("File: %s (%d lines, %d chars)\n", result.FilePath, result.TotalLines, result.TotalChars)
        fmt.Println(result.AgentHint)
    }
}
```

## 配置项

```go
webfetch.Config{
    // HTTP
    Timeout:       15 * time.Second,
    MaxRedirects:  5,
    UserAgent:     "...",

    // 安全
    BlockPrivateIP: true,  // SSRF 防护
    MaxURLLength:   2048,
    MaxBodyBytes:   5 * 1024 * 1024, // 5MB

    // 内容输出
    MaxInlineLines:   100,     // 超过此行数写文件
    MaxInlineChars:   0,       // 超过此字符数写文件
    MaxContentLength: 100000,  // 100KB 内容上限
    FileOutputDir:    "",      // 默认 os.TempDir()/webfetch/
    FileTTL:          24 * time.Hour,

    // 代理
    ProxyURL: "", // 支持 http/https/socks5
}
```

## 错误类型

| 类型 | 含义 |
|------|------|
| `*NotFoundError` | URL 返回 404/410，URL 不存在 |
| `*WAFError` | 被 WAF/反爬拦截 |
| `*EmptyContentError` | HTTP 200 但内容为空（反爬空壳页面） |
| `*SSRFError` | 目标解析到私有 IP |

## 适用场景

- **AI Agent 网页浏览**：结构化输出、大文件自动落盘、Agent 提示
- **内容采集管线**：高质量 Markdown 转换，保留表格/代码/列表结构
- **安全敏感环境**：SSRF 防护、WAF 检测、URL 校验
- **PDF 文档处理**：中文 PDF 解析，自动清理排版噪声

## 相比常见实现的优势

| 维度 | go-webfetch | 常见实现 |
|------|------------|---------|
| HTML→Markdown | readability + 启发式 + GFM 表格 | 正则剥标签 / html-to-text |
| 安全 | SSRF + WAF + 404 检测 | 无 |
| 大文件 | 流式写文件 + Agent 提示 | 全部内存 / 截断 |
| PDF | 解析 + 中文噪声清理 | 不支持 |
| 输出 | 结构化 JSON（title/method/mode/lines） | 纯文本 |

## License

Apache License 2.0
