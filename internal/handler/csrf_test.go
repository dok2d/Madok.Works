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
