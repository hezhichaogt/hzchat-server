/*
Package limiter provides concurrency rate limiting functionality based on IP addresses.

It utilizes the Token Bucket algorithm (rate.Limiter) to control the request frequency
for each client IP address and includes a cleanup goroutine to periodically remove
inactive limiters, preventing memory leaks.
*/
package limiter

import (
	"net"
	"net/http"
	"sync"
	"time"

	"hzchat/internal/pkg/errs"
	"hzchat/internal/pkg/logx"
	"hzchat/internal/pkg/resp"

	"golang.org/x/time/rate"
)

// IPRateLimiter implements a concurrency rate limiter based on client IP addresses.
type IPRateLimiter struct {
	// mu is used to protect concurrent access to the limits map.
	mu *sync.RWMutex

	// limits stores the map from client IP address to the *rate.Limiter instance.
	limits map[string]*rate.Limiter

	// r is the rate (rate.Limit) of the limiter, defining the number of events allowed per second.
	r rate.Limit

	// b is the burst size (token bucket size) of the limiter, defining the maximum burst of requests allowed.
	b int
}

// NewIPRateLimiter creates and returns a new IPRateLimiter instance.
// It accepts rate r and burst capacity b, and starts a background goroutine to periodically clean up inactive limiters.
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	i := &IPRateLimiter{
		mu:     &sync.RWMutex{},
		limits: make(map[string]*rate.Limiter),
		r:      r,
		b:      b,
	}

	go i.cleanUpVisitors()

	return i
}

// GetLimiter retrieves the rate limiter corresponding to the given IP address.
// If the limiter for that IP address does not exist, a new one is created and stored in the map.
// It uses a Double-Checked Locking pattern to ensure concurrent-safe creation of new limiters.
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.RLock()
	limiter, exists := i.limits[ip]
	i.mu.RUnlock()

	if !exists {
		i.mu.Lock()
		limiter, exists = i.limits[ip]
		if !exists {
			limiter = rate.NewLimiter(i.r, i.b)
			i.limits[ip] = limiter
		}
		i.mu.Unlock()
	}

	return limiter
}

// cleanUpVisitors periodically cleans up inactive rate limiters.
// An IP address is considered inactive and removed if its token bucket is full
// (i.e., tokens equal to the burst capacity), which frees up memory.
func (i *IPRateLimiter) cleanUpVisitors() {
	ticker := time.NewTicker(3 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		i.mu.Lock()
		count := 0
		for ip, limiter := range i.limits {
			if limiter.TokensAt(time.Now()) >= float64(limiter.Burst()) {
				delete(i.limits, ip)
				count++
			}
		}
		i.mu.Unlock()
		logx.Info("Rate limiter cleanup removed %d inactive IPs. %d active IPs remaining.", count, len(i.limits))
	}
}

// Middleware returns an HTTP middleware that performs rate limiting checks on incoming requests.
// If a request exceeds the limit, it responds with a 429 Too Many Requests error.
func (i *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		if ip == "" {
			ip = "unknown_ip"
		}

		limiter := i.GetLimiter(ip)

		if !limiter.Allow() {
			rateLimitErr := errs.NewError(errs.ErrRateLimitExceeded)
			resp.RespondError(w, r, rateLimitErr)
			return
		}

		next.ServeHTTP(w, r)
	})
}
