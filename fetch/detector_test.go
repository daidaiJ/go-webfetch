package fetch

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDetector_DetectCloudflare(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name       string
		status     int
		headers    map[string]string
		body       string
		wantDet    bool
		wantReason string
	}{
		{
			name:       "cloudflare 403",
			status:     403,
			headers:    map[string]string{"Server": "cloudflare"},
			body:       "<html><body>Access Denied</body></html>",
			wantDet:    true,
			wantReason: "waf_blocked:cloudflare",
		},
		{
			name:       "cloudflare challenge page",
			status:     503,
			headers:    map[string]string{"CF-RAY": "abc123"},
			body:       "<html><body>Just a moment...</body></html>",
			wantDet:    true,
			wantReason: "challenge_detected:",
		},
		{
			name:       "challenge keyword in body",
			status:     200,
			headers:    map[string]string{},
			body:       "<html><body>Checking your browser before accessing the site.</body></html>",
			wantDet:    true,
			wantReason: "challenge_detected:",
		},
		{
			name:       "chinese captcha",
			status:     200,
			headers:    map[string]string{},
			body:       "<html><body>请完成安全验证</body></html>",
			wantDet:    true,
			wantReason: "challenge_detected:",
		},
		{
			name:       "js required",
			status:     200,
			headers:    map[string]string{},
			body:       "<html><body>Please enable JavaScript to continue.</body></html>",
			wantDet:    true,
			wantReason: "js_render_required",
		},
		{
			name:       "datadome",
			status:     403,
			headers:    map[string]string{"X-DD-B": "1"},
			body:       "<html><body>Blocked</body></html>",
			wantDet:    true,
			wantReason: "waf_blocked:datadome",
		},
		{
			name:    "normal page",
			status:  200,
			headers: map[string]string{},
			body:    "<html><body><h1>Hello World</h1><p>Normal content here.</p></body></html>",
			wantDet: false,
		},
		{
			name:       "generic 403",
			status:     403,
			headers:    map[string]string{},
			body:       "<html><body>Forbidden</body></html>",
			wantDet:    true,
			wantReason: "waf_blocked:forbidden",
		},
		{
			name:       "rate limited",
			status:     429,
			headers:    map[string]string{},
			body:       "<html><body>Too Many Requests</body></html>",
			wantDet:    true,
			wantReason: "waf_blocked:rate_limited",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.status,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}
			for k, v := range tt.headers {
				resp.Header.Set(k, v)
			}

			detected, reason := d.Detect(resp, resp.Body)
			if detected != tt.wantDet {
				t.Errorf("Detect() = %v, want %v (reason: %s)", detected, tt.wantDet, reason)
			}
			if tt.wantDet && tt.wantReason != "" {
				if !strings.HasPrefix(reason, tt.wantReason) {
					t.Errorf("Detect() reason = %q, want prefix %q", reason, tt.wantReason)
				}
			}
		})
	}
}

func TestDetector_DoesNotFalsePositive(t *testing.T) {
	d := NewDetector()

	// 正常页面不应该被检测为 WAF
	normalBodies := []string{
		"<html><body><h1>Welcome</h1><p>Normal article content.</p></body></html>",
		"<html><body><p>This page requires you to login first.</p></body></html>", // login 提示不是 WAF
		strings.Repeat("<p>Content paragraph.</p>\n", 100),                        // 长内容
	}

	for i, body := range normalBodies {
		resp := &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(body)),
		}
		detected, reason := d.Detect(resp, resp.Body)
		if detected {
			t.Errorf("body %d: false positive detection: %s", i, reason)
		}
	}
}
