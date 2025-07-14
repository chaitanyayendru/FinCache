package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/chaitanyayendru/fincache/internal/config"
	"github.com/chaitanyayendru/fincache/internal/protocol"
	"github.com/chaitanyayendru/fincache/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type Server struct {
	config      *config.Config
	store       *store.Store
	logger      *zap.Logger
	httpServer  *http.Server
	redisServer *protocol.RedisServer
	metrics     *Metrics
}

type Metrics struct {
	requestsTotal     prometheus.Counter
	requestDuration   prometheus.Histogram
	activeConnections prometheus.Gauge
	storeSize         prometheus.Gauge
}

func NewServer(cfg *config.Config, store *store.Store, logger *zap.Logger) *Server {
	server := &Server{
		config: cfg,
		store:  store,
		logger: logger,
		metrics: &Metrics{
			requestsTotal: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "fincache_requests_total",
				Help: "Total number of requests",
			}),
			requestDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
				Name:    "fincache_request_duration_seconds",
				Help:    "Request duration in seconds",
				Buckets: prometheus.DefBuckets,
			}),
			activeConnections: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "fincache_active_connections",
				Help: "Number of active connections",
			}),
			storeSize: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "fincache_store_size",
				Help: "Number of keys in store",
			}),
		},
	}

	// Register metrics
	prometheus.MustRegister(
		server.metrics.requestsTotal,
		server.metrics.requestDuration,
		server.metrics.activeConnections,
		server.metrics.storeSize,
	)

	// Initialize Redis protocol server
	server.redisServer = protocol.NewRedisServer(store, logger)

	// Initialize HTTP server
	server.setupHTTPServer()

	return server
}

func (s *Server) setupHTTPServer() {
	if !s.config.API.Enabled {
		return
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(s.loggerMiddleware())
	router.Use(s.corsMiddleware())

	// Health check endpoint
	if s.config.Server.EnableHealth {
		router.GET("/health", s.healthHandler)
		router.GET("/ready", s.readyHandler)
	}

	// Metrics endpoint
	if s.config.Server.EnableMetrics {
		router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	// API endpoints
	api := router.Group("/api/v1")
	{
		api.GET("/keys/:key", s.getKeyHandler)
		api.POST("/keys/:key", s.setKeyHandler)
		api.DELETE("/keys/:key", s.deleteKeyHandler)
		api.GET("/keys", s.listKeysHandler)
		api.GET("/stats", s.statsHandler)
		api.POST("/flush", s.flushHandler)
		api.GET("/sandbox", s.sandboxHandler)
	}

	// WebSocket endpoint for real-time updates
	router.GET("/ws", s.websocketHandler)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.API.Port),
		Handler:      router,
		ReadTimeout:  s.config.API.ReadTimeout,
		WriteTimeout: s.config.API.WriteTimeout,
	}
}

func (s *Server) Start(host string, port int) error {
	// Start Redis protocol server
	go func() {
		addr := fmt.Sprintf("%s:%d", host, port)
		s.logger.Info("Starting Redis protocol server", zap.String("address", addr))

		if err := s.redisServer.Start(addr); err != nil {
			s.logger.Error("Redis server failed", zap.Error(err))
		}
	}()

	// Start HTTP server
	if s.config.API.Enabled {
		s.logger.Info("Starting HTTP server",
			zap.Int("port", s.config.API.Port))

		return s.httpServer.ListenAndServe()
	}

	// Wait indefinitely if only Redis server is enabled
	select {}
}

func (s *Server) Shutdown(ctx context.Context) error {
	var errors []error

	// Shutdown HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Errorf("HTTP server shutdown failed: %w", err))
		}
	}

	// Shutdown Redis server
	if s.redisServer != nil {
		if err := s.redisServer.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Errorf("Redis server shutdown failed: %w", err))
		}
	}

	// Close store
	if err := s.store.Close(); err != nil {
		errors = append(errors, fmt.Errorf("store close failed: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	return nil
}

// HTTP Handlers

func (s *Server) healthHandler(c *gin.Context) {
	s.metrics.requestsTotal.Inc()

	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	})
}

func (s *Server) readyHandler(c *gin.Context) {
	s.metrics.requestsTotal.Inc()

	// Check if store is ready
	stats := s.store.Stats()

	c.JSON(http.StatusOK, gin.H{
		"status":    "ready",
		"timestamp": time.Now().Unix(),
		"store": gin.H{
			"total_keys":   stats.TotalKeys,
			"memory_usage": stats.MemoryUsage,
		},
	})
}

func (s *Server) getKeyHandler(c *gin.Context) {
	start := time.Now()
	defer func() {
		s.metrics.requestDuration.Observe(time.Since(start).Seconds())
	}()

	s.metrics.requestsTotal.Inc()

	key := c.Param("key")
	value, err := s.store.Get(key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"key":   key,
		"value": value,
	})
}

func (s *Server) setKeyHandler(c *gin.Context) {
	start := time.Now()
	defer func() {
		s.metrics.requestDuration.Observe(time.Since(start).Seconds())
	}()

	s.metrics.requestsTotal.Inc()

	key := c.Param("key")

	var req struct {
		Value interface{} `json:"value" binding:"required"`
		TTL   int64       `json:"ttl,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var ttl time.Duration
	if req.TTL > 0 {
		ttl = time.Duration(req.TTL) * time.Second
	}

	if err := s.store.Set(key, req.Value, ttl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"key":    key,
	})
}

func (s *Server) deleteKeyHandler(c *gin.Context) {
	start := time.Now()
	defer func() {
		s.metrics.requestDuration.Observe(time.Since(start).Seconds())
	}()

	s.metrics.requestsTotal.Inc()

	key := c.Param("key")

	if err := s.store.Delete(key); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) listKeysHandler(c *gin.Context) {
	start := time.Now()
	defer func() {
		s.metrics.requestDuration.Observe(time.Since(start).Seconds())
	}()

	s.metrics.requestsTotal.Inc()

	pattern := c.Query("pattern")
	if pattern == "" {
		pattern = "*"
	}

	keys := s.store.Keys(pattern)
	c.JSON(http.StatusOK, gin.H{
		"keys":  keys,
		"count": len(keys),
	})
}

func (s *Server) statsHandler(c *gin.Context) {
	s.metrics.requestsTotal.Inc()

	stats := s.store.Stats()
	s.metrics.storeSize.Set(float64(stats.TotalKeys))

	c.JSON(http.StatusOK, stats)
}

func (s *Server) flushHandler(c *gin.Context) {
	start := time.Now()
	defer func() {
		s.metrics.requestDuration.Observe(time.Since(start).Seconds())
	}()

	s.metrics.requestsTotal.Inc()

	if err := s.store.Flush(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) sandboxHandler(c *gin.Context) {
	s.metrics.requestsTotal.Inc()

	// Return sandbox information and example commands
	c.JSON(http.StatusOK, gin.H{
		"message": "FinCache Sandbox",
		"examples": gin.H{
			"redis_cli": []string{
				"redis-cli -h localhost -p 6379 SET mykey myvalue",
				"redis-cli -h localhost -p 6379 GET mykey",
				"redis-cli -h localhost -p 6379 SETEX mykey 60 myvalue",
				"redis-cli -h localhost -p 6379 TTL mykey",
			},
			"http_api": []string{
				"curl -X POST http://localhost:8080/api/v1/keys/mykey -H 'Content-Type: application/json' -d '{\"value\":\"myvalue\"}'",
				"curl http://localhost:8080/api/v1/keys/mykey",
				"curl -X DELETE http://localhost:8080/api/v1/keys/mykey",
				"curl http://localhost:8080/api/v1/stats",
			},
		},
		"endpoints": gin.H{
			"health":    "/health",
			"metrics":   "/metrics",
			"api":       "/api/v1",
			"websocket": "/ws",
		},
	})
}

func (s *Server) websocketHandler(c *gin.Context) {
	// WebSocket implementation for real-time updates
	c.JSON(http.StatusOK, gin.H{
		"message": "WebSocket endpoint - implementation pending",
	})
}

// Middleware

func (s *Server) loggerMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		s.logger.Info("HTTP Request",
			zap.String("method", param.Method),
			zap.String("path", param.Path),
			zap.Int("status", param.StatusCode),
			zap.Duration("latency", param.Latency),
			zap.String("client_ip", param.ClientIP),
		)
		return ""
	})
}

func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.config.API.CORSEnabled {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}

		c.Next()
	}
}
