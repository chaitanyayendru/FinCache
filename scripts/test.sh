#!/bin/bash

# FinCache Test Script
# Tests both Redis protocol and HTTP API functionality

set -e

echo "🧪 FinCache Test Suite"
echo "====================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
FINCACHE_HOST="localhost"
FINCACHE_PORT="6379"
API_PORT="8080"

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Function to run test
run_test() {
    local test_name="$1"
    local command="$2"
    local expected="$3"
    
    echo -n "Testing $test_name... "
    
    if eval "$command" | grep -q "$expected"; then
        echo -e "${GREEN}✓ PASS${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}✗ FAIL${NC}"
        ((TESTS_FAILED++))
    fi
}

# Function to test HTTP API
test_http_api() {
    local test_name="$1"
    local method="$2"
    local endpoint="$3"
    local data="$4"
    local expected="$5"
    
    echo -n "Testing HTTP API $test_name... "
    
    local response
    if [ -n "$data" ]; then
        response=$(curl -s -X $method "http://$FINCACHE_HOST:$API_PORT$endpoint" \
            -H "Content-Type: application/json" \
            -d "$data")
    else
        response=$(curl -s -X $method "http://$FINCACHE_HOST:$API_PORT$endpoint")
    fi
    
    if echo "$response" | grep -q "$expected"; then
        echo -e "${GREEN}✓ PASS${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}✗ FAIL${NC}"
        echo "Expected: $expected"
        echo "Got: $response"
        ((TESTS_FAILED++))
    fi
}

# Wait for services to be ready
wait_for_services() {
    echo -e "${YELLOW}Waiting for services to be ready...${NC}"
    
    # Wait for Redis protocol
    for i in {1..30}; do
        if redis-cli -h $FINCACHE_HOST -p $FINCACHE_PORT PING &> /dev/null; then
            break
        fi
        sleep 1
    done
    
    # Wait for HTTP API
    for i in {1..30}; do
        if curl -s "http://$FINCACHE_HOST:$API_PORT/health" &> /dev/null; then
            break
        fi
        sleep 1
    done
    
    echo -e "${GREEN}Services are ready!${NC}"
}

# Redis Protocol Tests
test_redis_protocol() {
    echo -e "\n${YELLOW}Testing Redis Protocol${NC}"
    echo "----------------------"
    
    # Basic commands
    run_test "PING" "redis-cli -h $FINCACHE_HOST -p $FINCACHE_PORT PING" "PONG"
    run_test "ECHO" "redis-cli -h $FINCACHE_HOST -p $FINCACHE_PORT ECHO 'hello'" "hello"
    
    # Key-value operations
    run_test "SET" "redis-cli -h $FINCACHE_HOST -p $FINCACHE_PORT SET testkey testvalue" "OK"
    run_test "GET" "redis-cli -h $FINCACHE_HOST -p $FINCACHE_PORT GET testkey" "testvalue"
    run_test "EXISTS" "redis-cli -h $FINCACHE_HOST -p $FINCACHE_PORT EXISTS testkey" "1"
    
    # TTL operations
    run_test "SETEX" "redis-cli -h $FINCACHE_HOST -p $FINCACHE_PORT SETEX ttlkey 60 ttlvalue" "OK"
    run_test "TTL" "redis-cli -h $FINCACHE_HOST -p $FINCACHE_PORT TTL ttlkey" "[0-9]"
    
    # Multiple operations
    run_test "MSET" "redis-cli -h $FINCACHE_HOST -p $FINCACHE_PORT MSET key1 value1 key2 value2" "OK"
    run_test "MGET" "redis-cli -h $FINCACHE_HOST -p $FINCACHE_PORT MGET key1 key2" "value1"
    
    # Cleanup
    redis-cli -h $FINCACHE_HOST -p $FINCACHE_PORT DEL testkey ttlkey key1 key2 &> /dev/null
}

# HTTP API Tests
test_http_api_endpoints() {
    echo -e "\n${YELLOW}Testing HTTP API${NC}"
    echo "----------------"
    
    # Health check
    test_http_api "Health Check" "GET" "/health" "" "healthy"
    
    # Key operations
    test_http_api "SET Key" "POST" "/api/v1/keys/testkey" '{"value":"testvalue"}' "ok"
    test_http_api "GET Key" "GET" "/api/v1/keys/testkey" "" "testvalue"
    test_http_api "DELETE Key" "DELETE" "/api/v1/keys/testkey" "" "ok"
    
    # Stats
    test_http_api "Stats" "GET" "/api/v1/stats" "" "total_keys"
    
    # Sandbox
    test_http_api "Sandbox" "GET" "/sandbox" "" "FinCache Sandbox"
    
    # Cleanup
    curl -s -X DELETE "http://$FINCACHE_HOST:$API_PORT/api/v1/keys/testkey" &> /dev/null
}

# Performance Tests
test_performance() {
    echo -e "\n${YELLOW}Testing Performance${NC}"
    echo "-------------------"
    
    # Simple performance test
    echo -n "Testing SET performance (1000 operations)... "
    start_time=$(date +%s.%N)
    
    for i in {1..1000}; do
        redis-cli -h $FINCACHE_HOST -p $FINCACHE_PORT SET "perfkey$i" "value$i" &> /dev/null
    done
    
    end_time=$(date +%s.%N)
    duration=$(echo "$end_time - $start_time" | bc)
    ops_per_sec=$(echo "scale=2; 1000 / $duration" | bc)
    
    echo -e "${GREEN}✓ $ops_per_sec ops/sec${NC}"
    
    # Cleanup
    redis-cli -h $FINCACHE_HOST -p $FINCACHE_PORT FLUSHDB &> /dev/null
}

# Main test execution
main() {
    echo "Starting FinCache tests..."
    
    # Wait for services
    wait_for_services
    
    # Run tests
    test_redis_protocol
    test_http_api_endpoints
    test_performance
    
    # Summary
    echo -e "\n${YELLOW}Test Summary${NC}"
    echo "------------"
    echo -e "${GREEN}Tests Passed: $TESTS_PASSED${NC}"
    echo -e "${RED}Tests Failed: $TESTS_FAILED${NC}"
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "\n${GREEN}🎉 All tests passed!${NC}"
        exit 0
    else
        echo -e "\n${RED}❌ Some tests failed!${NC}"
        exit 1
    fi
}

# Check dependencies
check_dependencies() {
    if ! command -v redis-cli &> /dev/null; then
        echo -e "${RED}Error: redis-cli not found. Please install Redis tools.${NC}"
        exit 1
    fi
    
    if ! command -v curl &> /dev/null; then
        echo -e "${RED}Error: curl not found. Please install curl.${NC}"
        exit 1
    fi
    
    if ! command -v bc &> /dev/null; then
        echo -e "${RED}Error: bc not found. Please install bc.${NC}"
        exit 1
    fi
}

# Run main function
check_dependencies
main "$@" 