package guard

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateURL 校验 URL 格式
func ValidateURL(rawURL string, maxLen int) error {
	if maxLen > 0 && len(rawURL) > maxLen {
		return fmt.Errorf("URL too long: %d > %d", len(rawURL), maxLen)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s (only http/https)", scheme)
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("URL has no hostname")
	}
	return nil
}

// CheckPrivateIP 解析 URL 的 IP 并检查是否为内网/保留地址
func CheckPrivateIP(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	host := parsed.Hostname()

	// 先尝试作为 IP 直接解析
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrReserved(ip) {
			return &privateIPError{host: host, ip: ip.String()}
		}
		return nil
	}

	// 域名解析
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("DNS lookup failed for %s: %w", host, err)
	}

	for _, addr := range ips {
		if isPrivateOrReserved(addr.IP) {
			return &privateIPError{host: host, ip: addr.IP.String()}
		}
	}
	return nil
}

func isPrivateOrReserved(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		isCGNAT(ip) ||
		ip.IsMulticast()
}

func isCGNAT(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127
	}
	return false
}

type privateIPError struct {
	host string
	ip   string
}

func (e *privateIPError) Error() string {
	return fmt.Sprintf("SSRF blocked: %s resolved to private IP %s", e.host, e.ip)
}
