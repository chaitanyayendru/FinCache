package protocol

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

type PubSubManager struct {
	mu       sync.RWMutex
	channels map[string]*Channel
	patterns map[string]*Pattern
	logger   *zap.Logger
}

type Channel struct {
	mu          sync.RWMutex
	name        string
	subscribers map[string]*Subscriber
	lastMessage *Message
}

type Pattern struct {
	mu          sync.RWMutex
	pattern     string
	subscribers map[string]*Subscriber
}

type Subscriber struct {
	id       string
	conn     *RedisConnection
	channels map[string]bool
	patterns map[string]bool
	lastSeen time.Time
}

type Message struct {
	Channel   string
	Pattern   string
	Payload   string
	Timestamp time.Time
}

type RedisConnection struct {
	id         string
	writer     *ResponseWriter
	subscribed bool
}

type ResponseWriter struct {
	write func([]byte) error
}

func NewPubSubManager(logger *zap.Logger) *PubSubManager {
	psm := &PubSubManager{
		channels: make(map[string]*Channel),
		patterns: make(map[string]*Pattern),
		logger:   logger,
	}

	// Start cleanup goroutine
	go psm.cleanupExpiredSubscribers()

	return psm
}

func (psm *PubSubManager) Subscribe(connID, channelName string, writer *ResponseWriter) error {
	psm.mu.Lock()
	defer psm.mu.Unlock()

	// Create channel if it doesn't exist
	if _, exists := psm.channels[channelName]; !exists {
		psm.channels[channelName] = &Channel{
			name:        channelName,
			subscribers: make(map[string]*Subscriber),
		}
	}

	channel := psm.channels[channelName]
	channel.mu.Lock()
	defer channel.mu.Unlock()

	// Create or update subscriber
	subscriber := &Subscriber{
		id:       connID,
		conn:     &RedisConnection{id: connID, writer: writer, subscribed: true},
		channels: make(map[string]bool),
		lastSeen: time.Now(),
	}

	subscriber.channels[channelName] = true
	channel.subscribers[connID] = subscriber

	psm.logger.Info("Subscriber joined channel",
		zap.String("conn_id", connID),
		zap.String("channel", channelName))

	return nil
}

func (psm *PubSubManager) PSubscribe(connID, pattern string, writer *ResponseWriter) error {
	psm.mu.Lock()
	defer psm.mu.Unlock()

	// Create pattern if it doesn't exist
	if _, exists := psm.patterns[pattern]; !exists {
		psm.patterns[pattern] = &Pattern{
			pattern:     pattern,
			subscribers: make(map[string]*Subscriber),
		}
	}

	patternObj := psm.patterns[pattern]
	patternObj.mu.Lock()
	defer patternObj.mu.Unlock()

	// Create or update subscriber
	subscriber := &Subscriber{
		id:       connID,
		conn:     &RedisConnection{id: connID, writer: writer, subscribed: true},
		patterns: make(map[string]bool),
		lastSeen: time.Now(),
	}

	subscriber.patterns[pattern] = true
	patternObj.subscribers[connID] = subscriber

	psm.logger.Info("Subscriber joined pattern",
		zap.String("conn_id", connID),
		zap.String("pattern", pattern))

	return nil
}

func (psm *PubSubManager) Unsubscribe(connID, channelName string) error {
	psm.mu.Lock()
	defer psm.mu.Unlock()

	if channel, exists := psm.channels[channelName]; exists {
		channel.mu.Lock()
		defer channel.mu.Unlock()

		if subscriber, exists := channel.subscribers[connID]; exists {
			delete(subscriber.channels, channelName)
			delete(channel.subscribers, connID)

			// Remove channel if no subscribers
			if len(channel.subscribers) == 0 {
				delete(psm.channels, channelName)
			}

			psm.logger.Info("Subscriber left channel",
				zap.String("conn_id", connID),
				zap.String("channel", channelName))
		}
	}

	return nil
}

func (psm *PubSubManager) PUnsubscribe(connID, pattern string) error {
	psm.mu.Lock()
	defer psm.mu.Unlock()

	if patternObj, exists := psm.patterns[pattern]; exists {
		patternObj.mu.Lock()
		defer patternObj.mu.Unlock()

		if subscriber, exists := patternObj.subscribers[connID]; exists {
			delete(subscriber.patterns, pattern)
			delete(patternObj.subscribers, connID)

			// Remove pattern if no subscribers
			if len(patternObj.subscribers) == 0 {
				delete(psm.patterns, pattern)
			}

			psm.logger.Info("Subscriber left pattern",
				zap.String("conn_id", connID),
				zap.String("pattern", pattern))
		}
	}

	return nil
}

func (psm *PubSubManager) Publish(channelName, message string) int {
	psm.mu.RLock()
	defer psm.mu.RUnlock()

	msg := &Message{
		Channel:   channelName,
		Payload:   message,
		Timestamp: time.Now(),
	}

	recipients := 0

	// Send to direct channel subscribers
	if channel, exists := psm.channels[channelName]; exists {
		channel.mu.RLock()
		for _, subscriber := range channel.subscribers {
			if err := psm.sendMessage(subscriber, msg); err == nil {
				recipients++
			}
		}
		channel.mu.RUnlock()

		// Update last message
		channel.mu.Lock()
		channel.lastMessage = msg
		channel.mu.Unlock()
	}

	// Send to pattern subscribers
	for pattern, patternObj := range psm.patterns {
		if psm.matchPattern(pattern, channelName) {
			patternObj.mu.RLock()
			for _, subscriber := range patternObj.subscribers {
				if err := psm.sendMessage(subscriber, msg); err == nil {
					recipients++
				}
			}
			patternObj.mu.RUnlock()
		}
	}

	psm.logger.Info("Message published",
		zap.String("channel", channelName),
		zap.String("message", message),
		zap.Int("recipients", recipients))

	return recipients
}

func (psm *PubSubManager) sendMessage(subscriber *Subscriber, msg *Message) error {
	// Format message according to Redis Pub/Sub protocol
	var response string
	if msg.Pattern != "" {
		response = fmt.Sprintf("*3\r\n$8\r\npmessage\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n",
			len(msg.Pattern), msg.Pattern,
			len(msg.Channel), msg.Channel,
			len(msg.Payload), msg.Payload)
	} else {
		response = fmt.Sprintf("*3\r\n$7\r\nmessage\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n",
			len(msg.Channel), msg.Channel,
			len(msg.Payload), msg.Payload)
	}

	// Update last seen
	subscriber.lastSeen = time.Now()

	// Send message
	return subscriber.conn.writer.write([]byte(response))
}

func (psm *PubSubManager) matchPattern(pattern, channel string) bool {
	// Simple pattern matching (can be enhanced with regex)
	if pattern == "*" {
		return true
	}
	if pattern == channel {
		return true
	}
	// Add more pattern matching logic here
	return false
}

func (psm *PubSubManager) GetChannels(pattern string) []string {
	psm.mu.RLock()
	defer psm.mu.RUnlock()

	var channels []string
	for channelName := range psm.channels {
		if pattern == "*" || psm.matchPattern(pattern, channelName) {
			channels = append(channels, channelName)
		}
	}
	return channels
}

func (psm *PubSubManager) GetNumSub(channelName string) int {
	psm.mu.RLock()
	defer psm.mu.RUnlock()

	if channel, exists := psm.channels[channelName]; exists {
		channel.mu.RLock()
		defer channel.mu.RUnlock()
		return len(channel.subscribers)
	}
	return 0
}

func (psm *PubSubManager) cleanupExpiredSubscribers() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		psm.mu.Lock()
		now := time.Now()

		// Clean up channel subscribers
		for channelName, channel := range psm.channels {
			channel.mu.Lock()
			for connID, subscriber := range channel.subscribers {
				if now.Sub(subscriber.lastSeen) > 30*time.Minute {
					delete(channel.subscribers, connID)
					psm.logger.Info("Removed expired subscriber from channel",
						zap.String("conn_id", connID),
						zap.String("channel", channelName))
				}
			}
			if len(channel.subscribers) == 0 {
				delete(psm.channels, channelName)
			}
			channel.mu.Unlock()
		}

		// Clean up pattern subscribers
		for pattern, patternObj := range psm.patterns {
			patternObj.mu.Lock()
			for connID, subscriber := range patternObj.subscribers {
				if now.Sub(subscriber.lastSeen) > 30*time.Minute {
					delete(patternObj.subscribers, connID)
					psm.logger.Info("Removed expired subscriber from pattern",
						zap.String("conn_id", connID),
						zap.String("pattern", pattern))
				}
			}
			if len(patternObj.subscribers) == 0 {
				delete(psm.patterns, pattern)
			}
			patternObj.mu.Unlock()
		}

		psm.mu.Unlock()
	}
}
