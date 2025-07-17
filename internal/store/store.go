package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chaitanyayendru/fincache/internal/config"
	"go.uber.org/zap"
)

type Store struct {
	mu         sync.RWMutex
	data       map[string]*Item
	ttl        map[string]time.Time
	sortedSets map[string]*SortedSet
	config     config.StoreConfig
	logger     *zap.Logger
	ctx        context.Context
	cancel     context.CancelFunc
}

type Item struct {
	Value       interface{} `json:"value"`
	Type        string      `json:"type"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	ExpiresAt   *time.Time  `json:"expires_at,omitempty"`
	AccessCount int64       `json:"access_count"`
}

type StoreStats struct {
	TotalKeys   int64   `json:"total_keys"`
	MemoryUsage int64   `json:"memory_usage"`
	HitRate     float64 `json:"hit_rate"`
	MissRate    float64 `json:"miss_rate"`
	Evictions   int64   `json:"evictions"`
	ExpiredKeys int64   `json:"expired_keys"`
}

func NewStore(cfg config.StoreConfig) *Store {
	ctx, cancel := context.WithCancel(context.Background())

	store := &Store{
		data:       make(map[string]*Item),
		ttl:        make(map[string]time.Time),
		sortedSets: make(map[string]*SortedSet),
		config:     cfg,
		ctx:        ctx,
		cancel:     cancel,
	}

	// Start TTL cleanup goroutine if enabled
	if cfg.TTLEnabled {
		go store.cleanupExpiredKeys()
	}

	// Start snapshot goroutine if enabled
	if cfg.SnapshotEnabled {
		go store.snapshotWorker()
	}

	return store
}

func (s *Store) Set(key string, value interface{}, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var expiresAt *time.Time

	if ttl > 0 {
		exp := now.Add(ttl)
		expiresAt = &exp
		s.ttl[key] = exp
	}

	item := &Item{
		Value:       value,
		Type:        getType(value),
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   expiresAt,
		AccessCount: 0,
	}

	s.data[key] = item
	return nil
}

func (s *Store) Get(key string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, exists := s.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	// Check if expired
	if item.ExpiresAt != nil && time.Now().After(*item.ExpiresAt) {
		return nil, fmt.Errorf("key expired: %s", key)
	}

	// Update access count and timestamp
	item.AccessCount++
	item.UpdatedAt = time.Now()

	return item.Value, nil
}

func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[key]; !exists {
		return fmt.Errorf("key not found: %s", key)
	}

	delete(s.data, key)
	delete(s.ttl, key)
	return nil
}

func (s *Store) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, exists := s.data[key]
	if !exists {
		return false
	}

	// Check if expired
	if item.ExpiresAt != nil && time.Now().After(*item.ExpiresAt) {
		return false
	}

	return true
}

func (s *Store) Keys(pattern string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	now := time.Now()

	for key, item := range s.data {
		// Check if expired
		if item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
			continue
		}

		// Simple pattern matching (can be enhanced with regex)
		if pattern == "*" || key == pattern {
			keys = append(keys, key)
		}
	}

	return keys
}

func (s *Store) TTL(key string) (time.Duration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, exists := s.data[key]
	if !exists {
		return -2, fmt.Errorf("key not found: %s", key)
	}

	if item.ExpiresAt == nil {
		return -1, nil // No TTL
	}

	ttl := time.Until(*item.ExpiresAt)
	if ttl <= 0 {
		return -2, nil // Expired
	}

	return ttl, nil
}

func (s *Store) Expire(key string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.data[key]
	if !exists {
		return fmt.Errorf("key not found: %s", key)
	}

	expiresAt := time.Now().Add(ttl)
	item.ExpiresAt = &expiresAt
	item.UpdatedAt = time.Now()
	s.ttl[key] = expiresAt

	return nil
}

func (s *Store) Stats() *StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &StoreStats{
		TotalKeys: int64(len(s.data)),
	}

	// Calculate memory usage (rough estimation)
	for _, item := range s.data {
		if item.ExpiresAt != nil && time.Now().After(*item.ExpiresAt) {
			stats.ExpiredKeys++
		}
	}

	return stats
}

func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string]*Item)
	s.ttl = make(map[string]time.Time)
	return nil
}

func (s *Store) cleanupExpiredKeys() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			var expiredKeys []string

			for key, item := range s.data {
				if item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
					expiredKeys = append(expiredKeys, key)
				}
			}

			for _, key := range expiredKeys {
				delete(s.data, key)
				delete(s.ttl, key)
			}
			s.mu.Unlock()

			if len(expiredKeys) > 0 {
				s.logger.Info("Cleaned up expired keys",
					zap.Int("count", len(expiredKeys)))
			}
		}
	}
}

func (s *Store) snapshotWorker() {
	ticker := time.NewTicker(s.config.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.SaveSnapshot(); err != nil {
				s.logger.Error("Failed to save snapshot", zap.Error(err))
			}
		}
	}
}

func (s *Store) SaveSnapshot() error {
	s.mu.RLock()
	data := make(map[string]*Item)
	for k, v := range s.data {
		data[k] = v
	}
	s.mu.RUnlock()

	// Save to file (implement file I/O)
	return nil
}

func (s *Store) LoadSnapshot() error {
	// Load from file (implement file I/O)
	return nil
}

// Sorted Set Methods
func (s *Store) ZAdd(key string, score float64, member string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sortedSets[key]; !exists {
		s.sortedSets[key] = NewSortedSet()
	}

	return s.sortedSets[key].ZAdd(key, score, member)
}

func (s *Store) ZRem(key string, members ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sortedSet, exists := s.sortedSets[key]; exists {
		return sortedSet.ZRem(key, members...)
	}
	return 0
}

func (s *Store) ZScore(key string, member string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sortedSet, exists := s.sortedSets[key]; exists {
		return sortedSet.ZScore(key, member)
	}
	return 0, false
}

func (s *Store) ZRank(key string, member string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sortedSet, exists := s.sortedSets[key]; exists {
		return sortedSet.ZRank(key, member)
	}
	return -1
}

func (s *Store) ZRevRank(key string, member string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sortedSet, exists := s.sortedSets[key]; exists {
		return sortedSet.ZRevRank(key, member)
	}
	return -1
}

func (s *Store) ZRange(key string, start, stop int) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sortedSet, exists := s.sortedSets[key]; exists {
		return sortedSet.ZRange(key, start, stop)
	}
	return []string{}
}

func (s *Store) ZRevRange(key string, start, stop int) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sortedSet, exists := s.sortedSets[key]; exists {
		return sortedSet.ZRevRange(key, start, stop)
	}
	return []string{}
}

func (s *Store) ZCard(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sortedSet, exists := s.sortedSets[key]; exists {
		return sortedSet.ZCard(key)
	}
	return 0
}

func (s *Store) ZIncrBy(key string, increment float64, member string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sortedSets[key]; !exists {
		s.sortedSets[key] = NewSortedSet()
	}

	return s.sortedSets[key].ZIncrBy(key, increment, member)
}

// Order Book specific methods
func (s *Store) GetOrderBook(key string, depth int) ([]*SortedSetMember, []*SortedSetMember) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sortedSet, exists := s.sortedSets[key]; exists {
		return sortedSet.GetOrderBook(depth)
	}
	return []*SortedSetMember{}, []*SortedSetMember{}
}

func (s *Store) GetBestBid(key string) (*SortedSetMember, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sortedSet, exists := s.sortedSets[key]; exists {
		return sortedSet.GetBestBid()
	}
	return nil, false
}

func (s *Store) GetBestAsk(key string) (*SortedSetMember, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sortedSet, exists := s.sortedSets[key]; exists {
		return sortedSet.GetBestAsk()
	}
	return nil, false
}

func (s *Store) GetSpread(key string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sortedSet, exists := s.sortedSets[key]; exists {
		return sortedSet.GetSpread()
	}
	return 0, false
}

func (s *Store) Close() error {
	s.cancel()
	return nil
}

func getType(value interface{}) string {
	switch value.(type) {
	case string:
		return "string"
	case int, int32, int64:
		return "integer"
	case float32, float64:
		return "float"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "unknown"
	}
}
