package handler

import (
	"net/http"
	"testing"
)

func TestSameOriginOK(t *testing.T) {
	mk := func(method, origin, referer string) *http.Request {
		r, _ := http.NewRequest(method, "http://kanban.local/api/x", nil)
		r.Host = "kanban.local"
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if referer != "" {
			r.Header.Set("Referer", referer)
		}
		return r
	}
	cases := []struct {
		name string
		req  *http.Request
		want bool
	}{
		{"GET always ok", mk("GET", "http://evil.test", ""), true},
		{"POST same origin", mk("POST", "http://kanban.local", ""), true},
		{"POST cross origin", mk("POST", "http://evil.test", ""), false},
		{"POST no headers (curl)", mk("POST", "", ""), true},
		{"POST referer same host", mk("POST", "", "http://kanban.local/login"), true},
		{"POST referer cross host", mk("POST", "", "http://evil.test/x"), false},
		{"DELETE cross origin", mk("DELETE", "http://evil.test", ""), false},
	}
	for _, c := range cases {
		if got := sameOriginOK(c.req); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

// Behind a reverse proxy the upstream Host can drop the port (nginx $host) while
// the browser Origin keeps it. CSRF must still pass for the same hostname.
func TestSameOriginBehindProxy(t *testing.T) {
	mk := func(host, origin, xfh string) *http.Request {
		r, _ := http.NewRequest("POST", "http://x/api/x", nil)
		r.Host = host
		r.Header.Set("Origin", origin)
		if xfh != "" {
			r.Header.Set("X-Forwarded-Host", xfh)
		}
		return r
	}
	cases := []struct {
		name string
		req  *http.Request
		want bool
	}{
		{"nginx Host without port, Origin with port", mk("192.168.143.101", "http://192.168.143.101:8089", ""), true},
		{"exact match with port", mk("192.168.143.101:8089", "http://192.168.143.101:8089", ""), true},
		{"X-Forwarded-Host honored", mk("127.0.0.1:8080", "http://kanban.example.com", "kanban.example.com"), true},
		{"different host still rejected", mk("192.168.143.101", "http://evil.test:8089", ""), false},
	}
	for _, c := range cases {
		if got := sameOriginOK(c.req); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
