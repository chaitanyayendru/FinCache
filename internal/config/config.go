package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Store  StoreConfig  `yaml:"store"`
	Redis  RedisConfig  `yaml:"redis"`
	API    APIConfig    `yaml:"api"`
}

type ServerConfig struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	MaxConnections int           `yaml:"max_connections"`
	EnableMetrics  bool          `yaml:"enable_metrics"`
	EnableHealth   bool          `yaml:"enable_health"`
}

type StoreConfig struct {
	MaxMemory        string        `yaml:"max_memory"`
	EvictionPolicy   string        `yaml:"eviction_policy"`
	TTLEnabled       bool          `yaml:"ttl_enabled"`
	SnapshotEnabled  bool          `yaml:"snapshot_enabled"`
	SnapshotPath     string        `yaml:"snapshot_path"`
	SnapshotInterval time.Duration `yaml:"snapshot_interval"`
}

type RedisConfig struct {
	Enabled      bool          `yaml:"enabled"`
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	Password     string        `yaml:"password"`
	DB           int           `yaml:"db"`
	PoolSize     int           `yaml:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns"`
	MaxRetries   int           `yaml:"max_retries"`
	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type APIConfig struct {
	Enabled      bool          `yaml:"enabled"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	CORSEnabled  bool          `yaml:"cors_enabled"`
	RateLimit    int           `yaml:"rate_limit"`
}

func Load(path string) (*Config, error) {
	// Try to load from file first
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		var config Config
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}

		return &config, nil
	}

	// Fallback to environment-based config
	return loadFromEnv(), nil
}

func loadFromEnv() *Config {
	port, _ := strconv.Atoi(getEnv("FINCACHE_PORT", "6379"))
	apiPort, _ := strconv.Atoi(getEnv("FINCACHE_API_PORT", "8080"))
	maxConnections, _ := strconv.Atoi(getEnv("FINCACHE_MAX_CONNECTIONS", "10000"))
	poolSize, _ := strconv.Atoi(getEnv("FINCACHE_REDIS_POOL_SIZE", "10"))
	rateLimit, _ := strconv.Atoi(getEnv("FINCACHE_RATE_LIMIT", "1000"))

	return &Config{
		Server: ServerConfig{
			Host:           getEnv("FINCACHE_HOST", "0.0.0.0"),
			Port:           port,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			MaxConnections: maxConnections,
			EnableMetrics:  getEnv("FINCACHE_ENABLE_METRICS", "true") == "true",
			EnableHealth:   getEnv("FINCACHE_ENABLE_HEALTH", "true") == "true",
		},
		Store: StoreConfig{
			MaxMemory:        getEnv("FINCACHE_MAX_MEMORY", "1GB"),
			EvictionPolicy:   getEnv("FINCACHE_EVICTION_POLICY", "lru"),
			TTLEnabled:       getEnv("FINCACHE_TTL_ENABLED", "true") == "true",
			SnapshotEnabled:  getEnv("FINCACHE_SNAPSHOT_ENABLED", "true") == "true",
			SnapshotPath:     getEnv("FINCACHE_SNAPSHOT_PATH", "./data/snapshot.rdb"),
			SnapshotInterval: 5 * time.Minute,
		},
		Redis: RedisConfig{
			Enabled:      getEnv("FINCACHE_REDIS_ENABLED", "false") == "true",
			Host:         getEnv("FINCACHE_REDIS_HOST", "localhost"),
			Port:         6379,
			Password:     getEnv("FINCACHE_REDIS_PASSWORD", ""),
			DB:           0,
			PoolSize:     poolSize,
			MinIdleConns: 5,
			MaxRetries:   3,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
		API: APIConfig{
			Enabled:      getEnv("FINCACHE_API_ENABLED", "true") == "true",
			Port:         apiPort,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			CORSEnabled:  getEnv("FINCACHE_CORS_ENABLED", "true") == "true",
			RateLimit:    rateLimit,
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
