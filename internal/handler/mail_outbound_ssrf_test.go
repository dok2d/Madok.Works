package handler

import (
	"net"
	"testing"
)

func TestIsInternalIP(t *testing.T) {
	internal := []string{"127.0.0.1", "10.0.0.5", "192.168.1.1", "172.16.0.1", "169.254.1.1", "::1", "0.0.0.0"}
	public := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"}
	for _, s := range internal {
		if !isInternalIP(net.ParseIP(s)) {
			t.Errorf("%s should be internal", s)
		}
	}
	for _, s := range public {
		if isInternalIP(net.ParseIP(s)) {
			t.Errorf("%s should be public", s)
		}
	}
}
