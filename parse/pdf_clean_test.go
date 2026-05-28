package parse

import (
	"testing"
)

func TestCleanPDFText(t *testing.T) {
	input := "1\n\n北京青云科技股份有限公司\n\n-\n2023\n年\n7\n月\n-\n\n致新员工的欢迎词\n\n亲爱的\n新同事\n：\n\n欢迎\n您\n加入"
	out := cleanPDFText(input)
	t.Logf("Output:\n%s", out)

	// 关键断言：单字合并
	assertions := []struct {
		name string
		want string
	}{
		{"company name merged", "北京青云科技股份有限公司"},
		{"date merged", "2023年7月"},
		{"greeting merged", "亲爱的新同事："},
		{"welcome merged", "欢迎您加入"},
	}
	for _, a := range assertions {
		if !containsLine(out, a.want) {
			t.Errorf("missing %s: want line containing %q", a.name, a.want)
		}
	}
}

func containsLine(text, sub string) bool {
	for _, line := range splitLines(text) {
		if contains(line, sub) {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && findSubstring(s, sub)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
