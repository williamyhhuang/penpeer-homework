package geoip_test

import (
	"context"
	"testing"

	"github.com/penpeer/shortlink/application/geoip"
)

func TestLookupCountry_PrivateIP_ReturnsEmpty(t *testing.T) {
	privateIPs := []string{
		"127.0.0.1",     // loopback
		"192.168.1.1",   // private Class C
		"10.0.0.1",      // private Class A
		"172.16.0.1",    // private Class B
		"::1",           // IPv6 loopback
		"invalid-ip",    // 非法格式
	}

	for _, ip := range privateIPs {
		result := geoip.LookupCountry(context.Background(), ip)
		if result != "" {
			t.Errorf("私有/非法 IP %q 應回傳空字串，got %q", ip, result)
		}
	}
}
