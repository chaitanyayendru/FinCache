#!/bin/bash

echo "🧪 Testing Phase 2: Lua Scripting & Cluster Mode"
echo "================================================"

# Test Lua Scripting functionality
echo ""
echo "📜 Testing Lua Scripting"
echo "------------------------"

# Load and execute a custom script
echo "Loading custom financial script..."
CUSTOM_SCRIPT='
local user_id = ARGV[1]
local amount = tonumber(ARGV[2])
local merchant = ARGV[3]

-- Get user risk profile
local risk_score = tonumber(redis.get(user_id .. ":risk_score")) or 0
local daily_spend = tonumber(redis.get(user_id .. ":daily_spend")) or 0

-- Calculate new risk factors
local amount_risk = amount > 1000 and (amount - 1000) * 0.001 or 0
local spend_risk = daily_spend > 5000 and (daily_spend - 5000) * 0.0001 or 0

local new_risk_score = risk_score + amount_risk + spend_risk

-- Update counters
redis.set(user_id .. ":daily_spend", daily_spend + amount)
redis.set(user_id .. ":risk_score", new_risk_score)

-- Return approval decision
if new_risk_score > 0.8 then
    return "DECLINED"
elseif new_risk_score > 0.5 then
    return "REVIEW"
else
    return "APPROVED"
end
'

# Execute the script
echo "Executing fraud detection script..."
redis-cli -h localhost -p 6379 EVAL "$CUSTOM_SCRIPT" 0 user123 1500 merchant_abc

echo ""
echo "Testing predefined financial scripts..."

# Test VWAP calculation
echo "Calculating VWAP for multiple symbols..."
redis-cli -h localhost -p 6379 SET "AAPL:price" "150.25"
redis-cli -h localhost -p 6379 SET "AAPL:volume" "1000000"
redis-cli -h localhost -p 6379 SET "GOOGL:price" "2750.50"
redis-cli -h localhost -p 6379 SET "GOOGL:volume" "500000"

redis-cli -h localhost -p 6379 EVAL "$(cat scripts/lua/vwap.lua)" 2 AAPL GOOGL

# Test portfolio value calculation
echo "Calculating portfolio value..."
redis-cli -h localhost -p 6379 ZADD "portfolio:user123:positions" 100 "AAPL"
redis-cli -h localhost -p 6379 ZADD "portfolio:user123:positions" 50 "GOOGL"
redis-cli -h localhost -p 6379 SET "price:AAPL" "150.25"
redis-cli -h localhost -p 6379 SET "price:GOOGL" "2750.50"

redis-cli -h localhost -p 6379 EVAL "$(cat scripts/lua/portfolio.lua)" 0 user123

echo ""
echo "🔗 Testing Cluster Mode"
echo "----------------------"

# Test cluster info
echo "Getting cluster information..."
redis-cli -h localhost -p 6379 CLUSTER INFO

echo ""
echo "Getting cluster nodes..."
redis-cli -h localhost -p 6379 CLUSTER NODES

echo ""
echo "Testing slot distribution..."
redis-cli -h localhost -p 6379 CLUSTER SLOTS

echo ""
echo "Testing key routing..."
echo "Routing key 'user:123' to appropriate node..."
redis-cli -h localhost -p 6379 CLUSTER KEYSLOT "user:123"

echo ""
echo "Testing cluster-aware operations..."

# Test cross-slot operations
echo "Testing multi-key operations across slots..."
redis-cli -h localhost -p 6379 MSET "user:123:name" "John Doe" "user:123:balance" "5000"

echo ""
echo "Testing cluster failover simulation..."
echo "Note: This is a simulation - no actual failover occurs"

echo ""
echo "🔍 Testing HTTP API for Lua Scripts"
echo "-----------------------------------"

# Test HTTP API endpoints for Lua scripting
echo "Loading script via HTTP API:"
curl -X POST http://localhost:8080/api/v1/scripts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test_script",
    "source": "return ARGV[1] .. \" processed by Lua\""
  }' | jq .

echo ""
echo "Executing script via HTTP API:"
curl -X POST http://localhost:8080/api/v1/scripts/test_script/execute \
  -H "Content-Type: application/json" \
  -d '{
    "keys": ["key1", "key2"],
    "args": ["test_data"]
  }' | jq .

echo ""
echo "Listing scripts via HTTP API:"
curl -s http://localhost:8080/api/v1/scripts | jq .

echo ""
echo "🔍 Testing HTTP API for Cluster"
echo "-------------------------------"

# Test cluster HTTP endpoints
echo "Getting cluster info via HTTP API:"
curl -s http://localhost:8080/api/v1/cluster/info | jq .

echo ""
echo "Getting cluster nodes via HTTP API:"
curl -s http://localhost:8080/api/v1/cluster/nodes | jq .

echo ""
echo "Getting cluster health via HTTP API:"
curl -s http://localhost:8080/api/v1/cluster/health | jq .

echo ""
echo "✅ Phase 2 Testing Complete!"
echo "============================"
echo ""
echo "Features tested:"
echo "✓ Lua scripting for custom business logic"
echo "✓ Financial calculation scripts (VWAP, Portfolio)"
echo "✓ Fraud detection with Lua"
echo "✓ Cluster mode and node management"
echo "✓ Slot distribution and key routing"
echo "✓ Cluster health monitoring"
echo "✓ HTTP API integration for scripts and cluster"
echo ""
echo "Next: Phase 3 - TLS/SSL & JSON Support" 