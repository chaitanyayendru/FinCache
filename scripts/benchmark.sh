#!/bin/bash

# FinCache Benchmark Script
# Compares FinCache performance against Redis

set -e

echo "🏦 FinCache Benchmark Suite"
echo "=========================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
FINCACHE_HOST="localhost"
FINCACHE_PORT="6379"
REDIS_HOST="localhost"
REDIS_PORT="6380"
ITERATIONS=10000
CONCURRENT=50

# Check if redis-benchmark is available
if ! command -v redis-benchmark &> /dev/null; then
    echo -e "${RED}Error: redis-benchmark not found. Please install Redis tools.${NC}"
    exit 1
fi

# Function to run benchmark
run_benchmark() {
    local name=$1
    local host=$2
    local port=$3
    
    echo -e "\n${YELLOW}Running benchmark for $name ($host:$port)${NC}"
    echo "----------------------------------------"
    
    # Basic SET/GET benchmark
    redis-benchmark -h $host -p $port -n $ITERATIONS -c $CONCURRENT -t SET,GET
    
    # Pipeline benchmark
    echo -e "\n${YELLOW}Pipeline benchmark (1000 requests, 50 connections)${NC}"
    redis-benchmark -h $host -p $port -n 1000 -c 50 -P 10 -t SET,GET
    
    # Memory usage
    echo -e "\n${YELLOW}Memory usage:${NC}"
    redis-cli -h $host -p $port INFO memory | grep used_memory_human
}

# Wait for services to be ready
wait_for_service() {
    local host=$1
    local port=$2
    local name=$3
    
    echo -e "${YELLOW}Waiting for $name to be ready...${NC}"
    for i in {1..30}; do
        if redis-cli -h $host -p $port PING &> /dev/null; then
            echo -e "${GREEN}$name is ready!${NC}"
            return 0
        fi
        sleep 1
    done
    echo -e "${RED}Timeout waiting for $name${NC}"
    return 1
}

# Main benchmark execution
main() {
    echo "Starting benchmarks..."
    
    # Wait for services
    wait_for_service $FINCACHE_HOST $FINCACHE_PORT "FinCache"
    wait_for_service $REDIS_HOST $REDIS_PORT "Redis"
    
    # Run benchmarks
    run_benchmark "FinCache" $FINCACHE_HOST $FINCACHE_PORT
    run_benchmark "Redis" $REDIS_HOST $REDIS_PORT
    
    echo -e "\n${GREEN}Benchmark completed!${NC}"
    echo -e "\n${YELLOW}Results Summary:${NC}"
    echo "- FinCache: http://localhost:8080/stats"
    echo "- Redis: redis-cli -h localhost -p 6380 INFO"
    echo "- Grafana: http://localhost:3000 (admin/admin)"
    echo "- Prometheus: http://localhost:9090"
}

# Run main function
main "$@" 