package protocol

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/chaitanyayendru/fincache/internal/store"
	"go.uber.org/zap"
)

type RedisServer struct {
	store  *store.Store
	logger *zap.Logger
	ctx    context.Context
	cancel context.CancelFunc
}

type RedisCommand struct {
	Name   string
	Args   []string
	Client net.Conn
}

func NewRedisServer(store *store.Store, logger *zap.Logger) *RedisServer {
	ctx, cancel := context.WithCancel(context.Background())

	return &RedisServer{
		store:  store,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (rs *RedisServer) Start(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start Redis server: %w", err)
	}
	defer listener.Close()

	rs.logger.Info("Redis server listening", zap.String("address", addr))

	for {
		select {
		case <-rs.ctx.Done():
			return nil
		default:
			conn, err := listener.Accept()
			if err != nil {
				rs.logger.Error("Failed to accept connection", zap.Error(err))
				continue
			}

			go rs.handleConnection(conn)
		}
	}
}

func (rs *RedisServer) Shutdown(ctx context.Context) error {
	rs.cancel()
	return nil
}

func (rs *RedisServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	rs.logger.Info("New Redis client connected",
		zap.String("remote_addr", conn.RemoteAddr().String()))

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		select {
		case <-rs.ctx.Done():
			return
		default:
			// Set read timeout
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))

			command, err := rs.readCommand(reader)
			if err != nil {
				rs.logger.Error("Failed to read command", zap.Error(err))
				rs.writeError(writer, "ERR "+err.Error())
				return
			}

			if command == nil {
				continue
			}

			response := rs.executeCommand(command)
			rs.writeResponse(writer, response)
			writer.Flush()
		}
	}
}

func (rs *RedisServer) readCommand(reader *bufio.Reader) (*RedisCommand, error) {
	// Read the first line (number of arguments)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "*") {
		return nil, fmt.Errorf("invalid RESP format")
	}

	numArgs, err := strconv.Atoi(line[1:])
	if err != nil {
		return nil, fmt.Errorf("invalid number of arguments")
	}

	if numArgs < 1 {
		return nil, fmt.Errorf("no command specified")
	}

	var args []string
	for i := 0; i < numArgs; i++ {
		// Read the length line
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "$") {
			return nil, fmt.Errorf("invalid RESP format")
		}

		argLen, err := strconv.Atoi(line[1:])
		if err != nil {
			return nil, fmt.Errorf("invalid argument length")
		}

		// Read the argument
		arg := make([]byte, argLen)
		_, err = reader.Read(arg)
		if err != nil {
			return nil, err
		}

		// Read the newline
		_, err = reader.ReadByte()
		if err != nil {
			return nil, err
		}

		args = append(args, string(arg))
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("no command specified")
	}

	return &RedisCommand{
		Name: strings.ToUpper(args[0]),
		Args: args[1:],
	}, nil
}

func (rs *RedisServer) executeCommand(cmd *RedisCommand) interface{} {
	switch cmd.Name {
	case "PING":
		return "PONG"
	case "ECHO":
		if len(cmd.Args) == 0 {
			return fmt.Errorf("ERR wrong number of arguments for 'echo' command")
		}
		return cmd.Args[0]
	case "SET":
		return rs.handleSet(cmd)
	case "GET":
		return rs.handleGet(cmd)
	case "DEL":
		return rs.handleDel(cmd)
	case "EXISTS":
		return rs.handleExists(cmd)
	case "KEYS":
		return rs.handleKeys(cmd)
	case "TTL":
		return rs.handleTTL(cmd)
	case "EXPIRE":
		return rs.handleExpire(cmd)
	case "FLUSHDB":
		return rs.handleFlushDB(cmd)
	case "INFO":
		return rs.handleInfo(cmd)
	case "QUIT":
		return "OK"
	default:
		return fmt.Errorf("ERR unknown command '%s'", cmd.Name)
	}
}

func (rs *RedisServer) handleSet(cmd *RedisCommand) interface{} {
	if len(cmd.Args) < 2 {
		return fmt.Errorf("ERR wrong number of arguments for 'set' command")
	}

	key := cmd.Args[0]
	value := cmd.Args[1]
	var ttl time.Duration

	// Check for TTL options
	for i := 2; i < len(cmd.Args); i++ {
		switch strings.ToUpper(cmd.Args[i]) {
		case "EX":
			if i+1 >= len(cmd.Args) {
				return fmt.Errorf("ERR wrong number of arguments for 'set' command")
			}
			seconds, err := strconv.Atoi(cmd.Args[i+1])
			if err != nil {
				return fmt.Errorf("ERR value is not an integer or out of range")
			}
			ttl = time.Duration(seconds) * time.Second
			i++ // Skip the next argument
		case "PX":
			if i+1 >= len(cmd.Args) {
				return fmt.Errorf("ERR wrong number of arguments for 'set' command")
			}
			milliseconds, err := strconv.Atoi(cmd.Args[i+1])
			if err != nil {
				return fmt.Errorf("ERR value is not an integer or out of range")
			}
			ttl = time.Duration(milliseconds) * time.Millisecond
			i++ // Skip the next argument
		}
	}

	err := rs.store.Set(key, value, ttl)
	if err != nil {
		return fmt.Errorf("ERR %v", err)
	}

	return "OK"
}

func (rs *RedisServer) handleGet(cmd *RedisCommand) interface{} {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("ERR wrong number of arguments for 'get' command")
	}

	key := cmd.Args[0]
	value, err := rs.store.Get(key)
	if err != nil {
		return nil // Redis returns nil for non-existent keys
	}

	return value
}

func (rs *RedisServer) handleDel(cmd *RedisCommand) interface{} {
	if len(cmd.Args) < 1 {
		return fmt.Errorf("ERR wrong number of arguments for 'del' command")
	}

	deleted := 0
	for _, key := range cmd.Args {
		err := rs.store.Delete(key)
		if err == nil {
			deleted++
		}
	}

	return deleted
}

func (rs *RedisServer) handleExists(cmd *RedisCommand) interface{} {
	if len(cmd.Args) < 1 {
		return fmt.Errorf("ERR wrong number of arguments for 'exists' command")
	}

	exists := 0
	for _, key := range cmd.Args {
		if rs.store.Exists(key) {
			exists++
		}
	}

	return exists
}

func (rs *RedisServer) handleKeys(cmd *RedisCommand) interface{} {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("ERR wrong number of arguments for 'keys' command")
	}

	pattern := cmd.Args[0]
	keys := rs.store.Keys(pattern)

	// Return as array
	return keys
}

func (rs *RedisServer) handleTTL(cmd *RedisCommand) interface{} {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("ERR wrong number of arguments for 'ttl' command")
	}

	key := cmd.Args[0]
	ttl, err := rs.store.TTL(key)
	if err != nil {
		return -2 // Key doesn't exist
	}

	if ttl == -1 {
		return -1 // No TTL
	}

	return int(ttl.Seconds())
}

func (rs *RedisServer) handleExpire(cmd *RedisCommand) interface{} {
	if len(cmd.Args) != 2 {
		return fmt.Errorf("ERR wrong number of arguments for 'expire' command")
	}

	key := cmd.Args[0]
	seconds, err := strconv.Atoi(cmd.Args[1])
	if err != nil {
		return fmt.Errorf("ERR value is not an integer or out of range")
	}

	err = rs.store.Expire(key, time.Duration(seconds)*time.Second)
	if err != nil {
		return 0 // Key doesn't exist
	}

	return 1 // Success
}

func (rs *RedisServer) handleFlushDB(cmd *RedisCommand) interface{} {
	err := rs.store.Flush()
	if err != nil {
		return fmt.Errorf("ERR %v", err)
	}

	return "OK"
}

func (rs *RedisServer) handleInfo(cmd *RedisCommand) interface{} {
	stats := rs.store.Stats()

	info := fmt.Sprintf(`# Server
redis_version:1.0.0
os:Go
tcp_port:6379
uptime_in_seconds:%d
total_connections_received:0
total_commands_processed:0

# Keyspace
db0:keys=%d,expires=0,avg_ttl=0

# Memory
used_memory:%d
used_memory_human:%d
used_memory_peak:%d
used_memory_peak_human:%d
`,
		time.Now().Unix(),
		stats.TotalKeys,
		stats.MemoryUsage,
		stats.MemoryUsage,
		stats.MemoryUsage,
		stats.MemoryUsage,
	)

	return info
}

func (rs *RedisServer) writeResponse(writer *bufio.Writer, response interface{}) {
	switch v := response.(type) {
	case string:
		rs.writeSimpleString(writer, v)
	case int:
		rs.writeInteger(writer, v)
	case []string:
		rs.writeArray(writer, v)
	case nil:
		rs.writeNull(writer)
	case error:
		rs.writeError(writer, v.Error())
	default:
		rs.writeBulkString(writer, fmt.Sprintf("%v", v))
	}
}

func (rs *RedisServer) writeSimpleString(writer *bufio.Writer, s string) {
	writer.WriteString("+" + s + "\r\n")
}

func (rs *RedisServer) writeError(writer *bufio.Writer, err string) {
	writer.WriteString("-" + err + "\r\n")
}

func (rs *RedisServer) writeInteger(writer *bufio.Writer, i int) {
	writer.WriteString(":" + strconv.Itoa(i) + "\r\n")
}

func (rs *RedisServer) writeBulkString(writer *bufio.Writer, s string) {
	writer.WriteString("$" + strconv.Itoa(len(s)) + "\r\n")
	writer.WriteString(s + "\r\n")
}

func (rs *RedisServer) writeNull(writer *bufio.Writer) {
	writer.WriteString("$-1\r\n")
}

func (rs *RedisServer) writeArray(writer *bufio.Writer, arr []string) {
	writer.WriteString("*" + strconv.Itoa(len(arr)) + "\r\n")
	for _, item := range arr {
		rs.writeBulkString(writer, item)
	}
}
