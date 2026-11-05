package middleware

import (
	"math"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Decision adalah hasil pemeriksaan rate limit.
type Decision struct {
	Allowed           bool
	Limit             int
	Remaining         int
	ResetSeconds      int
	RetryAfterSeconds int
}

// Limiter membatasi permintaan per kunci di memori Go. Kuncinya bisa akun, IP, atau order.
// Saat load balancer aktif, limiter ini diganti Redis lewat interface yang sama.
type Limiter struct {
	perMinute int
	now       func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
	swept   time.Time
}

type bucket struct {
	limiter *rate.Limiter
	seen    time.Time
}

const idleAfter = 10 * time.Minute

// NewLimiter membuat limiter dengan batas per menit.
func NewLimiter(perMinute int, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{perMinute: perMinute, now: now, buckets: map[string]*bucket{}, swept: now()}
}

// Allow memeriksa satu permintaan untuk kunci ini.
func (l *Limiter) Allow(key string) Decision {
	now := l.now()
	perSecond := rate.Limit(float64(l.perMinute) / 60)

	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.swept) > idleAfter {
		for k, b := range l.buckets {
			if now.Sub(b.seen) > idleAfter {
				delete(l.buckets, k)
			}
		}
		l.swept = now
	}
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(perSecond, l.perMinute)}
		l.buckets[key] = b
	}
	b.seen = now

	allowed := b.limiter.AllowN(now, 1)
	tokens := b.limiter.TokensAt(now)
	d := Decision{
		Allowed:      allowed,
		Limit:        l.perMinute,
		Remaining:    max(int(math.Floor(tokens)), 0),
		ResetSeconds: int(math.Ceil((float64(l.perMinute) - tokens) / float64(perSecond))),
	}
	if !allowed {
		d.RetryAfterSeconds = int(math.Ceil((1 - tokens) / float64(perSecond)))
	}
	return d
}

// Size mengembalikan jumlah kunci yang tersimpan.
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
