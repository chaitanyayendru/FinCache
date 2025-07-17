#!/bin/bash

echo "🧪 Testing Phase 1: Pub/Sub & Sorted Sets"
echo "=========================================="

# Test Pub/Sub functionality
echo ""
echo "📡 Testing Pub/Sub (Redis Protocol)"
echo "-----------------------------------"

# Start subscription in background
echo "Starting subscription to 'market-data' channel..."
redis-cli -h localhost -p 6379 SUBSCRIBE market-data &
SUB_PID=$!

# Wait a moment for subscription to be ready
sleep 2

# Publish messages
echo "Publishing market data updates..."
redis-cli -h localhost -p 6379 PUBLISH market-data "AAPL:150.25:1000000"
redis-cli -h localhost -p 6379 PUBLISH market-data "GOOGL:2750.50:500000"
redis-cli -h localhost -p 6379 PUBLISH market-data "MSFT:320.75:750000"

# Wait for messages to be received
sleep 3

# Kill subscription
kill $SUB_PID 2>/dev/null

echo ""
echo "📊 Testing Sorted Sets (Order Book)"
echo "-----------------------------------"

# Create order book for AAPL
echo "Creating AAPL order book..."

# Add bids (positive scores)
redis-cli -h localhost -p 6379 ZADD "orderbook:AAPL" 150.20 "bid:1000:user1"
redis-cli -h localhost -p 6379 ZADD "orderbook:AAPL" 150.15 "bid:500:user2"
redis-cli -h localhost -p 6379 ZADD "orderbook:AAPL" 150.10 "bid:750:user3"

# Add asks (negative scores for proper ordering)
redis-cli -h localhost -p 6379 ZADD "orderbook:AAPL" -150.25 "ask:300:user4"
redis-cli -h localhost -p 6379 ZADD "orderbook:AAPL" -150.30 "ask:600:user5"
redis-cli -h localhost -p 6379 ZADD "orderbook:AAPL" -150.35 "ask:400:user6"

echo "Order book created. Showing top 5 bids and asks:"
echo ""

# Show top bids (highest scores first)
echo "Top 5 Bids:"
redis-cli -h localhost -p 6379 ZREVRANGE "orderbook:AAPL" 0 4 WITHSCORES

echo ""
echo "Top 5 Asks:"
redis-cli -h localhost -p 6379 ZRANGE "orderbook:AAPL" 0 4 WITHSCORES

echo ""
echo "📈 Testing Leaderboards"
echo "----------------------"

# Create trading volume leaderboard
echo "Creating trading volume leaderboard..."

redis-cli -h localhost -p 6379 ZADD "leaderboard:volume" 1000000 "trader1"
redis-cli -h localhost -p 6379 ZADD "leaderboard:volume" 850000 "trader2"
redis-cli -h localhost -p 6379 ZADD "leaderboard:volume" 720000 "trader3"
redis-cli -h localhost -p 6379 ZADD "leaderboard:volume" 650000 "trader4"
redis-cli -h localhost -p 6379 ZADD "leaderboard:volume" 580000 "trader5"

echo "Top 5 Traders by Volume:"
redis-cli -h localhost -p 6379 ZREVRANGE "leaderboard:volume" 0 4 WITHSCORES

echo ""
echo "🏆 Testing Rankings"
echo "------------------"

# Check specific trader rankings
echo "Trader1 rank: $(redis-cli -h localhost -p 6379 ZREVRANK leaderboard:volume trader1)"
echo "Trader3 rank: $(redis-cli -h localhost -p 6379 ZREVRANK leaderboard:volume trader3)"
echo "Trader5 rank: $(redis-cli -h localhost -p 6379 ZREVRANK leaderboard:volume trader5)"

echo ""
echo "💰 Testing Score Updates"
echo "----------------------"

# Update trader scores
echo "Updating trader scores..."
redis-cli -h localhost -p 6379 ZINCRBY "leaderboard:volume" 50000 "trader2"
redis-cli -h localhost -p 6379 ZINCRBY "leaderboard:volume" 75000 "trader4"

echo "Updated Top 5 Traders:"
redis-cli -h localhost -p 6379 ZREVRANGE "leaderboard:volume" 0 4 WITHSCORES

echo ""
echo "🔍 Testing HTTP API for Sorted Sets"
echo "-----------------------------------"

# Test HTTP API endpoints
echo "Getting order book via HTTP API:"
curl -s http://localhost:8080/api/v1/sorted-sets/orderbook:AAPL/range/0/4 | jq .

echo ""
echo "Getting leaderboard via HTTP API:"
curl -s http://localhost:8080/api/v1/sorted-sets/leaderboard:volume/revrange/0/4 | jq .

echo ""
echo "✅ Phase 1 Testing Complete!"
echo "============================"
echo ""
echo "Features tested:"
echo "✓ Redis Pub/Sub for real-time market data"
echo "✓ Sorted Sets for order books"
echo "✓ Sorted Sets for leaderboards"
echo "✓ Ranking and scoring operations"
echo "✓ HTTP API integration"
echo ""
echo "Next: Phase 2 - Lua Scripting & Cluster Mode" 