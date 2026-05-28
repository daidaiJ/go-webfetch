package fetch

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

// Detector 反爬/WAF 检测器
type Detector struct{}

// NewDetector 创建检测器
func NewDetector() *Detector {
	return &Detector{}
}

// Detect 检测响应是否被 WAF/反爬拦截
// 返回 (是否拦截, 原因)
// 仅扫描响应头 + body 前 8KB，< 5ms 开销
func (d *Detector) Detect(resp *http.Response, body io.Reader) (bool, string) {
	// 读取 body 前 8KB 用于检测
	bodyPrefix := make([]byte, 8192)
	n, _ := io.ReadFull(body, bodyPrefix)
	bodyPrefix = bodyPrefix[:n]

	// 把读取的部分放回去，让后续处理可以正常读取
	if seeker, ok := body.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	} else {
		// 如果不能 seek，需要包装一下
		resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(bodyPrefix), body))
	}

	statusCode := resp.StatusCode
	server := strings.ToLower(resp.Header.Get("Server"))
	hasCFRay := resp.Header.Get("CF-RAY") != ""

	// 1. Cloudflare 检测
	if (statusCode == 403 || statusCode == 429) &&
		(strings.Contains(server, "cloudflare") || hasCFRay) {
		return true, "waf_blocked:cloudflare"
	}

	// 2. DataDome 检测
	if resp.Header.Get("X-DD-B") != "" || resp.Header.Get("X-Datadome") != "" {
		return true, "waf_blocked:datadome"
	}

	// 3. 通用 403/429
	if statusCode == 403 {
		return true, "waf_blocked:forbidden"
	}
	if statusCode == 429 {
		return true, "waf_blocked:rate_limited"
	}

	// 4. 响应体关键词检测
	if n > 0 {
		lower := strings.ToLower(string(bodyPrefix))
		challengeKeywords := []string{
			"challenge-platform",
			"turnstile",
			"jschl",
			"cf-chl-bypass",
			"checking your browser",
			"just a moment",
			"attention required",
			"安全验证",
			"验证码",
			"人机验证",
			"访问异常",
			"unusual traffic",
			"are you a human",
			"verify you are human",
		}
		for _, kw := range challengeKeywords {
			if strings.Contains(lower, kw) {
				return true, "challenge_detected:" + kw
			}
		}

		// 5. JS 渲染依赖检测
		jsKeywords := []string{
			"enable javascript",
			"requires javascript",
			"请启用 javascript",
			"请启用js",
		}
		for _, kw := range jsKeywords {
			if strings.Contains(lower, kw) {
				return true, "js_render_required"
			}
		}
	}

	return false, ""
}
