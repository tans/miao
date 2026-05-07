package middleware

import "testing"

func TestShouldBypassRateLimit(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/api/v1/admin/login", want: true},
		{path: "/api/v1/admin/users", want: true},
		{path: "/api/v1/admin/works/86", want: true},
		{path: "/api/v1/works", want: false},
		{path: "/api/v1/auth/wechat-mini-login", want: false},
	}

	for _, tt := range tests {
		if got := shouldBypassRateLimit(tt.path); got != tt.want {
			t.Fatalf("shouldBypassRateLimit(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
