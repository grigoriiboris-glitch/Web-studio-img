package privacy

import "testing"

func TestIsLocalEndpointHost(t *testing.T) {
	for _, host := range []string{"localhost", "LOCALHOST", "127.0.0.1", "::1", "127.0.0.42"} {
		if !isLocalEndpointHost(host) {
			t.Fatalf("expected %q to be local", host)
		}
	}
	for _, host := range []string{"example.com", "8.8.8.8", "192.168.1.10"} {
		if isLocalEndpointHost(host) {
			t.Fatalf("expected %q not to be treated as local", host)
		}
	}
}
