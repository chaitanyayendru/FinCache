package store

import (
	"testing"
	"time"

	"github.com/chaitanyayendru/fincache/internal/config"
)

func TestNewStore(t *testing.T) {
	cfg := config.StoreConfig{
		TTLEnabled:      true,
		SnapshotEnabled: true,
	}

	store := NewStore(cfg)
	if store == nil {
		t.Fatal("Expected store to be created")
	}

	// Test that store can be closed
	err := store.Close()
	if err != nil {
		t.Errorf("Expected no error when closing store: %v", err)
	}
}

func TestSetAndGet(t *testing.T) {
	store := NewStore(config.StoreConfig{})
	defer store.Close()

	// Test basic set and get
	err := store.Set("testkey", "testvalue", 0)
	if err != nil {
		t.Errorf("Expected no error when setting key: %v", err)
	}

	value, err := store.Get("testkey")
	if err != nil {
		t.Errorf("Expected no error when getting key: %v", err)
	}

	if value != "testvalue" {
		t.Errorf("Expected 'testvalue', got '%v'", value)
	}
}

func TestSetWithTTL(t *testing.T) {
	store := NewStore(config.StoreConfig{TTLEnabled: true})
	defer store.Close()

	// Test set with TTL
	err := store.Set("ttlkey", "ttlvalue", 1*time.Second)
	if err != nil {
		t.Errorf("Expected no error when setting key with TTL: %v", err)
	}

	// Should exist immediately
	value, err := store.Get("ttlkey")
	if err != nil {
		t.Errorf("Expected no error when getting key: %v", err)
	}

	if value != "ttlvalue" {
		t.Errorf("Expected 'ttlvalue', got '%v'", value)
	}

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Should not exist after TTL
	_, err = store.Get("ttlkey")
	if err == nil {
		t.Error("Expected error when getting expired key")
	}
}

func TestDelete(t *testing.T) {
	store := NewStore(config.StoreConfig{})
	defer store.Close()

	// Set a key
	err := store.Set("delkey", "delvalue", 0)
	if err != nil {
		t.Errorf("Expected no error when setting key: %v", err)
	}

	// Delete the key
	err = store.Delete("delkey")
	if err != nil {
		t.Errorf("Expected no error when deleting key: %v", err)
	}

	// Should not exist
	_, err = store.Get("delkey")
	if err == nil {
		t.Error("Expected error when getting deleted key")
	}
}

func TestExists(t *testing.T) {
	store := NewStore(config.StoreConfig{})
	defer store.Close()

	// Test non-existent key
	exists := store.Exists("nonexistent")
	if exists {
		t.Error("Expected non-existent key to return false")
	}

	// Set a key
	err := store.Set("existkey", "existvalue", 0)
	if err != nil {
		t.Errorf("Expected no error when setting key: %v", err)
	}

	// Test existing key
	exists = store.Exists("existkey")
	if !exists {
		t.Error("Expected existing key to return true")
	}
}

func TestKeys(t *testing.T) {
	store := NewStore(config.StoreConfig{})
	defer store.Close()

	// Set multiple keys
	keys := []string{"key1", "key2", "key3"}
	for _, key := range keys {
		err := store.Set(key, "value", 0)
		if err != nil {
			t.Errorf("Expected no error when setting key %s: %v", key, err)
		}
	}

	// Get all keys
	foundKeys := store.Keys("*")
	if len(foundKeys) < len(keys) {
		t.Errorf("Expected at least %d keys, got %d", len(keys), len(foundKeys))
	}
}

func TestTTL(t *testing.T) {
	store := NewStore(config.StoreConfig{TTLEnabled: true})
	defer store.Close()

	// Test key without TTL
	err := store.Set("nottlkey", "value", 0)
	if err != nil {
		t.Errorf("Expected no error when setting key: %v", err)
	}

	ttl, err := store.TTL("nottlkey")
	if err != nil {
		t.Errorf("Expected no error when getting TTL: %v", err)
	}

	if ttl != -1 {
		t.Errorf("Expected TTL -1 for key without TTL, got %v", ttl)
	}

	// Test key with TTL
	err = store.Set("ttlkey", "value", 60*time.Second)
	if err != nil {
		t.Errorf("Expected no error when setting key with TTL: %v", err)
	}

	ttl, err = store.TTL("ttlkey")
	if err != nil {
		t.Errorf("Expected no error when getting TTL: %v", err)
	}

	if ttl <= 0 || ttl > 60 {
		t.Errorf("Expected TTL between 0 and 60, got %v", ttl)
	}
}

func TestExpire(t *testing.T) {
	store := NewStore(config.StoreConfig{TTLEnabled: true})
	defer store.Close()

	// Set a key without TTL
	err := store.Set("expirekey", "value", 0)
	if err != nil {
		t.Errorf("Expected no error when setting key: %v", err)
	}

	// Set TTL
	err = store.Expire("expirekey", 60*time.Second)
	if err != nil {
		t.Errorf("Expected no error when setting TTL: %v", err)
	}

	// Check TTL
	ttl, err := store.TTL("expirekey")
	if err != nil {
		t.Errorf("Expected no error when getting TTL: %v", err)
	}

	if ttl <= 0 || ttl > 60 {
		t.Errorf("Expected TTL between 0 and 60, got %v", ttl)
	}
}

func TestFlush(t *testing.T) {
	store := NewStore(config.StoreConfig{})
	defer store.Close()

	// Set multiple keys
	keys := []string{"flush1", "flush2", "flush3"}
	for _, key := range keys {
		err := store.Set(key, "value", 0)
		if err != nil {
			t.Errorf("Expected no error when setting key %s: %v", key, err)
		}
	}

	// Flush all
	err := store.Flush()
	if err != nil {
		t.Errorf("Expected no error when flushing: %v", err)
	}

	// Check that all keys are gone
	for _, key := range keys {
		exists := store.Exists(key)
		if exists {
			t.Errorf("Expected key %s to not exist after flush", key)
		}
	}
}

func TestStats(t *testing.T) {
	store := NewStore(config.StoreConfig{})
	defer store.Close()

	// Set some keys
	for i := 0; i < 5; i++ {
		err := store.Set("statskey"+string(rune(i)), "value", 0)
		if err != nil {
			t.Errorf("Expected no error when setting key: %v", err)
		}
	}

	// Get stats
	stats := store.Stats()
	if stats.TotalKeys != 5 {
		t.Errorf("Expected 5 total keys, got %d", stats.TotalKeys)
	}
}
