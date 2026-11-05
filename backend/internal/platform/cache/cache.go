// Package cache menyediakan interface Cache dan implementasi memori Go.
package cache

import (
	"strings"
	"sync"
	"time"
)

// Cache adalah penyimpanan sementara. Saat load balancer aktif, implementasinya diganti Redis.
type Cache interface {
	Get(key string) (any, bool)
	Set(key string, value any, ttl time.Duration)
	DeletePrefix(prefix string)
}

type entry struct {
	value   any
	expires time.Time
}

// Memory adalah Cache di memori proses.
type Memory struct {
	now func() time.Time

	mu    sync.RWMutex
	items map[string]entry
}

const maxEntries = 10000

// NewMemory membuat cache kosong.
func NewMemory(now func() time.Time) *Memory {
	if now == nil {
		now = time.Now
	}
	return &Memory{now: now, items: map[string]entry{}}
}

// Get membaca nilai yang belum kedaluwarsa.
func (m *Memory) Get(key string) (any, bool) {
	m.mu.RLock()
	e, ok := m.items[key]
	m.mu.RUnlock()
	if !ok || !m.now().Before(e.expires) {
		return nil, false
	}
	return e.value, true
}

// Set menyimpan nilai dengan masa berlaku.
func (m *Memory) Set(key string, value any, ttl time.Duration) {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.items) >= maxEntries {
		for k, e := range m.items {
			if !now.Before(e.expires) {
				delete(m.items, k)
			}
		}
		if len(m.items) >= maxEntries {
			m.items = map[string]entry{}
		}
	}
	m.items[key] = entry{value: value, expires: now.Add(ttl)}
}

// DeletePrefix menghapus semua kunci dengan awalan tertentu.
func (m *Memory) DeletePrefix(prefix string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.items {
		if strings.HasPrefix(k, prefix) {
			delete(m.items, k)
		}
	}
}
