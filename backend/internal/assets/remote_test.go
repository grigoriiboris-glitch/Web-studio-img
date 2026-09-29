package assets

import (
	"context"
	"net"
	"testing"
)

func TestIsBlockedRemoteIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{name: "loopback", ip: "127.0.0.1", want: true},
		{name: "private", ip: "10.0.0.4", want: true},
		{name: "link local", ip: "169.254.1.1", want: true},
		{name: "ipv6 loopback", ip: "::1", want: true},
		{name: "multicast", ip: "224.0.0.1", want: true},
		{name: "public", ip: "8.8.8.8", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBlockedRemoteIP(net.ParseIP(tt.ip)); got != tt.want {
				t.Fatalf("isBlockedRemoteIP(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestValidateRemoteImportURLRejectsPrivateHosts(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1/image.png",
		"http://localhost/image.png",
		"http://10.0.0.1/image.png",
		"file:///tmp/image.png",
	} {
		if _, err := validateRemoteImportURL(context.Background(), raw); err == nil {
			t.Fatalf("validateRemoteImportURL(%q) unexpectedly succeeded", raw)
		}
	}
}
