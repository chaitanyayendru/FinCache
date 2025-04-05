# 🏦 FinCache

**FinCache** is a custom-built Redis-like in-memory key-value store developed in **Java Spring Boot**, with added support for financial applications and high-performance backend microservices.

> ⚡ Ultra-fast | 🛡️ Secure | 💸 Financial-Ready | ☁️ Microservice-Compatible

---

## 🚀 Features

- Custom RESP protocol handler (Test Driven)
- TCP Client-Server communication
- Redis-compatible CLI support
- Core commands: `PING`, `ECHO`, `SET`, `GET`
- Optional TTL with active/passive expiry
- Thread-safe design for concurrent clients
- Asynchronous background expiry
- Benchmark-ready with Redis CLI & `redis-benchmark`
- Snapshot-based persistence (write/load)
- Financial-specific RESP extensions (e.g. instruments, fraud tokens)
- Pub/Sub for market data simulation
- Sorted Sets for order books and leaderboard-style data
- Multi-key atomic transactions for secure operations

---

## 🧱 Architecture Overview

```
[Spring Boot API] → [RESP Protocol Handler] → [In-Memory Store]
                                        ↓
                               [Persistence Layer]
                                        ↓
                                 [Snapshot File]
```

- Multi-threaded client handling
- Layered services for extensibility
- Protocol abstraction layer to plug custom data types

---

## 🛠️ Setup Instructions

1. Clone this repo:
   ```bash
   git clone https://github.com/chaitanyayendru/fincache.git
   cd fincache
   ```

2. Start Redis (for testing):
   ```bash
   brew install redis
   redis-server
   ```

3. Run the server:
   ```bash
   ./gradlew bootRun
   ```

4. Interact using CLI:
   ```bash
   redis-cli -p 6379
   ```

---

## 📈 Benchmark Example

```bash
redis-benchmark -h localhost -p 6379 -n 10000 -c 50 -t SET,GET
```

✅ Expected results include <1ms latency under 10k req/sec for basic commands.

---

## 💡 Use Cases

- Fraud signal caching
- Market data publishing/subscription
- Real-time order book simulations
- Rate limiting engine
- Financial instrument caching
- Background compliance checks

---

## 📂 Project Structure

```
/src
  /main
    /java
      /com.fincache
        /protocol     # RESP handlers
        /server       # TCP Server logic
        /store        # In-memory store
        /persistence  # Snapshot save/load
        /financial    # Fintech-specific logic
    /resources
  /test
```

---

## 🎯 Roadmap

- [ ] Custom Redis Protocol support for financial metadata
- [ ] Streamlined async processing (Netty, Reactor)
- [ ] Redis Pub/Sub compatibility
- [ ] Cloud-native deployment (Docker, K8s)
- [ ] RedisJSON-style support for structured financial objects

---

## 👨‍💻 Contributing

We love PRs and feedback! Please follow conventional commits and add tests with your changes.

---

## 📄 License

MIT © [Chaitanya Yendru]

---
