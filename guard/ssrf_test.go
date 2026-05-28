package guard

import (
	"context"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		maxLen  int
		wantErr bool
	}{
		{"valid https", "https://example.com/path", 2048, false},
		{"valid http", "http://example.com", 2048, false},
		{"too long", "https://example.com/" + string(make([]byte, 3000)), 2048, true},
		{"no scheme", "example.com", 2048, true},
		{"ftp scheme", "ftp://example.com", 2048, true},
		{"javascript scheme", "javascript:alert(1)", 2048, true},
		{"empty host", "https://", 2048, true},
		{"valid with port", "https://example.com:8080/path", 2048, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url, tt.maxLen)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q, %d) error = %v, wantErr %v", tt.url, tt.maxLen, err, tt.wantErr)
			}
		})
	}
}

func TestCheckPrivateIP(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"public domain", "https://example.com", false},
		{"localhost", "http://localhost:8080", true},
		{"loopback ip", "http://127.0.0.1:8080", true},
		{"private 10.x", "http://10.0.0.1", true},
		{"private 192.168.x", "http://192.168.1.1", true},
		{"private 172.16.x", "http://172.16.0.1", true},
		{"cgnat 100.64.x", "http://100.64.0.1", true},
		{"link-local 169.254.x", "http://169.254.1.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPrivateIP(context.Background(), tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPrivateIP(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}
