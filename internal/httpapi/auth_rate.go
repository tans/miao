package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	authLoginLimit    = 10
	authEndpointLimit = 20
)

type authRateBucket struct {
	window time.Time
	count  int
}

var authRateMu sync.Mutex
var authRateBuckets = map[string]authRateBucket{}

func allowAuthRequest(w http.ResponseWriter, r *http.Request, limit int) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if parsed := net.ParseIP(ip); parsed != nil && parsed.IsLoopback() {
		if proxyIP := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); proxyIP != nil {
			ip = proxyIP.String()
		}
	}
	key := r.URL.Path + "\x00" + ip
	now := time.Now()
	authRateMu.Lock()
	bucket := authRateBuckets[key]
	if now.Sub(bucket.window) >= time.Minute {
		bucket = authRateBucket{window: now}
	}
	bucket.count++
	authRateBuckets[key] = bucket
	if len(authRateBuckets) > 4096 {
		for candidateKey, candidate := range authRateBuckets {
			if now.Sub(candidate.window) >= 2*time.Minute {
				delete(authRateBuckets, candidateKey)
			}
		}
	}
	authRateMu.Unlock()
	if bucket.count > limit {
		w.Header().Set("Retry-After", "60")
		w.Header().Set("Cache-Control", "no-store")
		writeError(w, http.StatusTooManyRequests, "请求过于频繁，请稍后重试")
		return false
	}
	return true
}
