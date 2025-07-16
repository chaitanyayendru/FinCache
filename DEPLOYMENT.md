# 🚀 FinCache Deployment Guide

This guide covers deploying FinCache in various environments, from local development to production.

## 📋 Prerequisites

### Required Tools
- **Docker & Docker Compose** (for containerized deployment)
- **Go 1.21+** (for local development)
- **Redis CLI** (for testing and benchmarking)
- **curl** (for API testing)

### Optional Tools
- **Make** (for using Makefile commands)
- **Prometheus & Grafana** (for monitoring)

## 🏠 Local Development

### Quick Start (Docker)
```bash
# Clone the repository
git clone https://github.com/chaitanyayendru/fincache.git
cd fincache

# Start all services
docker-compose up -d

# Verify services are running
docker-compose ps
```

### Manual Setup
```bash
# Install Go dependencies
go mod tidy

# Build the application
go build -o fincache cmd/fincache/main.go

# Run FinCache
./fincache

# Or run directly
go run cmd/fincache/main.go
```

### Using Makefile
```bash
# Setup development environment
make setup

# Build and run
make run

# Run tests
make test

# Run integration tests
make test-integration
```

## 🐳 Docker Deployment

### Single Container
```bash
# Build image
docker build -t fincache .

# Run container
docker run -d \
  --name fincache \
  -p 6379:6379 \
  -p 8080:8080 \
  fincache
```

### Docker Compose (Recommended)
```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f fincache

# Stop services
docker-compose down

# Clean up completely
docker-compose down -v --rmi all
```

### Production Docker
```bash
# Build production image
docker build --target production -t fincache:prod .

# Run with production settings
docker run -d \
  --name fincache-prod \
  -p 6379:6379 \
  -p 8080:8080 \
  -v /data/fincache:/app/data \
  -e FINCACHE_MAX_MEMORY=4GB \
  -e FINCACHE_RATE_LIMIT=5000 \
  --restart unless-stopped \
  fincache:prod
```

## ☁️ Cloud Deployment

### AWS ECS
```yaml
# task-definition.json
{
  "family": "fincache",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "1024",
  "memory": "2048",
  "executionRoleArn": "arn:aws:iam::account:role/ecsTaskExecutionRole",
  "containerDefinitions": [
    {
      "name": "fincache",
      "image": "fincache:latest",
      "portMappings": [
        {"containerPort": 6379, "protocol": "tcp"},
        {"containerPort": 8080, "protocol": "tcp"}
      ],
      "environment": [
        {"name": "FINCACHE_MAX_MEMORY", "value": "1GB"},
        {"name": "FINCACHE_RATE_LIMIT", "value": "1000"}
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/fincache",
          "awslogs-region": "us-east-1",
          "awslogs-stream-prefix": "ecs"
        }
      }
    }
  ]
}
```

### Google Cloud Run
```bash
# Build and push to Google Container Registry
docker build -t gcr.io/PROJECT_ID/fincache .
docker push gcr.io/PROJECT_ID/fincache

# Deploy to Cloud Run
gcloud run deploy fincache \
  --image gcr.io/PROJECT_ID/fincache \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --port 8080 \
  --memory 2Gi \
  --cpu 2
```

### Azure Container Instances
```bash
# Deploy to Azure Container Instances
az container create \
  --resource-group myResourceGroup \
  --name fincache \
  --image fincache:latest \
  --ports 6379 8080 \
  --environment-variables \
    FINCACHE_MAX_MEMORY=1GB \
    FINCACHE_RATE_LIMIT=1000 \
  --dns-name-label fincache \
  --location eastus
```

## 🏢 On-Premises Deployment

### System Requirements
- **CPU**: 2+ cores (4+ recommended for production)
- **Memory**: 4GB+ RAM (8GB+ recommended)
- **Storage**: 10GB+ available space
- **Network**: Low-latency network connection

### Installation Steps
```bash
# 1. Download and extract
wget https://github.com/chaitanyayendru/fincache/releases/latest/download/fincache-linux-amd64.tar.gz
tar -xzf fincache-linux-amd64.tar.gz
sudo mv fincache /usr/local/bin/

# 2. Create systemd service
sudo tee /etc/systemd/system/fincache.service << EOF
[Unit]
Description=FinCache Redis-compatible cache
After=network.target

[Service]
Type=simple
User=fincache
Group=fincache
ExecStart=/usr/local/bin/fincache
Restart=always
RestartSec=5
Environment=FINCACHE_MAX_MEMORY=2GB
Environment=FINCACHE_RATE_LIMIT=2000

[Install]
WantedBy=multi-user.target
EOF

# 3. Create user and directories
sudo useradd -r -s /bin/false fincache
sudo mkdir -p /var/lib/fincache /var/log/fincache
sudo chown fincache:fincache /var/lib/fincache /var/log/fincache

# 4. Start service
sudo systemctl daemon-reload
sudo systemctl enable fincache
sudo systemctl start fincache
```

## 🔧 Configuration

### Environment Variables
```bash
# Server configuration
FINCACHE_HOST=0.0.0.0
FINCACHE_PORT=6379
FINCACHE_API_PORT=8080

# Store configuration
FINCACHE_MAX_MEMORY=1GB
FINCACHE_EVICTION_POLICY=lru
FINCACHE_TTL_ENABLED=true

# API configuration
FINCACHE_ENABLE_METRICS=true
FINCACHE_ENABLE_HEALTH=true
FINCACHE_CORS_ENABLED=true
FINCACHE_RATE_LIMIT=1000
```

### Configuration File
```yaml
# config.yaml
server:
  host: "0.0.0.0"
  port: 6379
  read_timeout: 30s
  write_timeout: 30s
  max_connections: 10000
  enable_metrics: true
  enable_health: true

store:
  max_memory: "1GB"
  eviction_policy: "lru"
  ttl_enabled: true
  snapshot_enabled: true
  snapshot_path: "./data/snapshot.rdb"
  snapshot_interval: 5m

api:
  enabled: true
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  cors_enabled: true
  rate_limit: 1000
```

## 📊 Monitoring Setup

### Prometheus Configuration
```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'fincache'
    static_configs:
      - targets: ['fincache:8080']
    metrics_path: '/metrics'
    scrape_interval: 5s
```

### Grafana Dashboard
1. Access Grafana at http://localhost:3000
2. Login with admin/admin
3. Add Prometheus as data source
4. Import the FinCache dashboard from `monitoring/grafana/dashboards/`

### Health Checks
```bash
# Health endpoint
curl http://localhost:8080/health

# Readiness endpoint
curl http://localhost:8080/ready

# Metrics endpoint
curl http://localhost:8080/metrics
```

## 🔒 Security Considerations

### Network Security
```bash
# Firewall rules (iptables)
# Allow Redis protocol
iptables -A INPUT -p tcp --dport 6379 -j ACCEPT

# Allow HTTP API
iptables -A INPUT -p tcp --dport 8080 -j ACCEPT

# Block external access to metrics (optional)
iptables -A INPUT -p tcp --dport 8080 -s 127.0.0.1 -j ACCEPT
iptables -A INPUT -p tcp --dport 8080 -j DROP
```

### Container Security
```dockerfile
# Use non-root user
USER fincache

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
```

### TLS/SSL (Future Feature)
```bash
# Generate certificates
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes

# Configure TLS
FINCACHE_TLS_ENABLED=true
FINCACHE_TLS_CERT_FILE=cert.pem
FINCACHE_TLS_KEY_FILE=key.pem
```

## 🧪 Testing Deployment

### Basic Functionality
```bash
# Test Redis protocol
redis-cli -h localhost -p 6379 PING
redis-cli -h localhost -p 6379 SET testkey testvalue
redis-cli -h localhost -p 6379 GET testkey

# Test HTTP API
curl http://localhost:8080/health
curl -X POST http://localhost:8080/api/v1/keys/testkey \
  -H "Content-Type: application/json" \
  -d '{"value":"testvalue"}'
curl http://localhost:8080/api/v1/keys/testkey
```

### Performance Testing
```bash
# Run benchmarks
make benchmark

# Load testing
redis-benchmark -h localhost -p 6379 -n 100000 -c 100 -t SET,GET

# Memory usage
redis-cli -h localhost -p 6379 INFO memory
```

### Integration Testing
```bash
# Run integration tests
make test-integration

# Or manually
./scripts/test.sh  # Linux/Mac
scripts/test.bat   # Windows
```

## 🔄 Backup and Recovery

### Data Backup
```bash
# Create backup directory
mkdir -p /backup/fincache

# Backup configuration
cp config.yaml /backup/fincache/

# Backup data (if using persistence)
cp -r data/ /backup/fincache/

# Automated backup script
#!/bin/bash
BACKUP_DIR="/backup/fincache/$(date +%Y%m%d_%H%M%S)"
mkdir -p $BACKUP_DIR
cp config.yaml $BACKUP_DIR/
cp -r data/ $BACKUP_DIR/
tar -czf $BACKUP_DIR.tar.gz $BACKUP_DIR
rm -rf $BACKUP_DIR
```

### Recovery
```bash
# Stop FinCache
docker-compose down

# Restore data
tar -xzf backup_20231201_120000.tar.gz
cp -r backup_20231201_120000/data/ ./
cp backup_20231201_120000/config.yaml ./

# Restart FinCache
docker-compose up -d
```

## 🚨 Troubleshooting

### Common Issues

#### Service Won't Start
```bash
# Check logs
docker-compose logs fincache
journalctl -u fincache -f

# Check configuration
./fincache --help
./fincache --config config.yaml
```

#### High Memory Usage
```bash
# Check memory usage
redis-cli -h localhost -p 6379 INFO memory

# Reduce memory limit
export FINCACHE_MAX_MEMORY=512MB

# Enable eviction
export FINCACHE_EVICTION_POLICY=lru
```

#### Connection Issues
```bash
# Check if ports are open
netstat -tlnp | grep :6379
netstat -tlnp | grep :8080

# Test connectivity
telnet localhost 6379
curl http://localhost:8080/health
```

#### Performance Issues
```bash
# Check metrics
curl http://localhost:8080/metrics

# Monitor with Grafana
# Access http://localhost:3000

# Check system resources
htop
iostat
```

### Log Analysis
```bash
# View real-time logs
docker-compose logs -f fincache

# Search for errors
docker-compose logs fincache | grep ERROR

# Monitor specific endpoints
tail -f logs/fincache.log | grep "GET /api/v1/keys"
```

## 📈 Scaling

### Horizontal Scaling
```bash
# Multiple instances behind load balancer
docker-compose up -d --scale fincache=3

# Use Redis Sentinel for high availability
# (Future feature)
```

### Vertical Scaling
```bash
# Increase memory
export FINCACHE_MAX_MEMORY=4GB

# Increase CPU limits
docker run --cpus=4 --memory=4g fincache

# Optimize for high throughput
export FINCACHE_RATE_LIMIT=5000
```

## 🔄 Updates and Maintenance

### Rolling Updates
```bash
# Build new image
docker build -t fincache:v2 .

# Update one instance at a time
docker-compose up -d --no-deps --build fincache

# Verify health
curl http://localhost:8080/health
```

### Zero-Downtime Deployment
```bash
# Blue-green deployment
# 1. Deploy new version to green environment
# 2. Run health checks
# 3. Switch traffic
# 4. Decommission old version
```

## 📞 Support

### Getting Help
- **Documentation**: [README.md](README.md)
- **Issues**: [GitHub Issues](https://github.com/chaitanyayendru/fincache/issues)
- **Discussions**: [GitHub Discussions](https://github.com/chaitanyayendru/fincache/discussions)

### Monitoring and Alerts
```bash
# Set up alerts in Grafana
# Monitor key metrics:
# - Request rate
# - Response time
# - Memory usage
# - Error rate
# - Active connections
```

---

**FinCache** - Deploy with confidence! 🚀 