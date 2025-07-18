package scripting

import (
	"fmt"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

type LuaEngine struct {
	mu      sync.RWMutex
	state   *lua.LState
	scripts map[string]*LuaScript
	logger  interface{}
}

type LuaScript struct {
	Name      string
	Source    string
	Sha1      string
	CreatedAt time.Time
}

type ScriptResult struct {
	Success bool
	Result  interface{}
	Error   string
}

func NewLuaEngine(logger interface{}) *LuaEngine {
	L := lua.NewState()
	defer L.Close()

	engine := &LuaEngine{
		state:   L,
		scripts: make(map[string]*LuaScript),
		logger:  logger,
	}

	// Register custom functions
	engine.registerFunctions()

	return engine
}

func (le *LuaEngine) registerFunctions() {
	// Register Redis-like functions
	le.state.SetGlobal("redis", le.state.NewTable())
	redis := le.state.GetGlobal("redis").(*lua.LTable)

	// SET function
	redis.RawSetString("set", le.state.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		value := L.CheckString(2)
		// Implementation would call the actual store
		L.Push(lua.LString("OK"))
		return 1
	}))

	// GET function
	redis.RawSetString("get", le.state.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		// Implementation would call the actual store
		L.Push(lua.LString("value"))
		return 1
	}))

	// ZADD function
	redis.RawSetString("zadd", le.state.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		score := L.CheckNumber(2)
		member := L.CheckString(3)
		// Implementation would call the actual store
		L.Push(lua.LNumber(1))
		return 1
	}))

	// ZRANGE function
	redis.RawSetString("zrange", le.state.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		start := L.CheckInt(2)
		stop := L.CheckInt(3)
		// Implementation would call the actual store
		result := le.state.NewTable()
		L.Push(result)
		return 1
	}))

	// PUBLISH function
	redis.RawSetString("publish", le.state.NewFunction(func(L *lua.LState) int {
		channel := L.CheckString(1)
		message := L.CheckString(2)
		// Implementation would call the actual pub/sub
		L.Push(lua.LNumber(1))
		return 1
	}))

	// Math functions
	math := le.state.GetGlobal("math").(*lua.LTable)
	math.RawSetString("round", le.state.NewFunction(func(L *lua.LState) int {
		n := L.CheckNumber(1)
		L.Push(lua.LNumber(float64(int(n + 0.5))))
		return 1
	}))

	// Time functions
	le.state.SetGlobal("time", le.state.NewTable())
	timeTable := le.state.GetGlobal("time").(*lua.LTable)
	timeTable.RawSetString("now", le.state.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(time.Now().Unix()))
		return 1
	}))

	// JSON functions
	le.state.SetGlobal("json", le.state.NewTable())
	json := le.state.GetGlobal("json").(*lua.LTable)
	json.RawSetString("encode", le.state.NewFunction(func(L *lua.LState) int {
		// Simple JSON encoding
		table := L.CheckTable(1)
		result := "{"
		table.ForEach(func(key, value lua.LValue) {
			if result != "{" {
				result += ","
			}
			result += fmt.Sprintf("\"%v\":\"%v\"", key, value)
		})
		result += "}"
		L.Push(lua.LString(result))
		return 1
	}))

	// Financial functions
	le.state.SetGlobal("finance", le.state.NewTable())
	finance := le.state.GetGlobal("finance").(*lua.LTable)

	// Calculate moving average
	finance.RawSetString("moving_average", le.state.NewFunction(func(L *lua.LState) int {
		table := L.CheckTable(1)
		period := L.CheckInt(2)

		var values []float64
		table.ForEach(func(key, value lua.LValue) {
			if num, ok := value.(lua.LNumber); ok {
				values = append(values, float64(num))
			}
		})

		if len(values) < period {
			L.Push(lua.LNil)
			return 1
		}

		sum := 0.0
		for i := len(values) - period; i < len(values); i++ {
			sum += values[i]
		}
		average := sum / float64(period)

		L.Push(lua.LNumber(average))
		return 1
	}))

	// Calculate volatility
	finance.RawSetString("volatility", le.state.NewFunction(func(L *lua.LState) int {
		table := L.CheckTable(1)
		period := L.CheckInt(2)

		var values []float64
		table.ForEach(func(key, value lua.LValue) {
			if num, ok := value.(lua.LNumber); ok {
				values = append(values, float64(num))
			}
		})

		if len(values) < period {
			L.Push(lua.LNil)
			return 1
		}

		// Calculate mean
		sum := 0.0
		for i := len(values) - period; i < len(values); i++ {
			sum += values[i]
		}
		mean := sum / float64(period)

		// Calculate variance
		variance := 0.0
		for i := len(values) - period; i < len(values); i++ {
			diff := values[i] - mean
			variance += diff * diff
		}
		variance /= float64(period)

		// Calculate standard deviation (volatility)
		volatility := float64(int((variance*100)+0.5)) / 100

		L.Push(lua.LNumber(volatility))
		return 1
	}))

	// Calculate price change percentage
	finance.RawSetString("price_change", le.state.NewFunction(func(L *lua.LState) int {
		oldPrice := L.CheckNumber(1)
		newPrice := L.CheckNumber(2)

		if oldPrice == 0 {
			L.Push(lua.LNumber(0))
			return 1
		}

		change := ((newPrice - oldPrice) / oldPrice) * 100
		L.Push(lua.LNumber(change))
		return 1
	}))
}

func (le *LuaEngine) LoadScript(name, source string) error {
	le.mu.Lock()
	defer le.mu.Unlock()

	// Validate script
	if err := le.state.DoString(source); err != nil {
		return fmt.Errorf("invalid script: %v", err)
	}

	script := &LuaScript{
		Name:      name,
		Source:    source,
		Sha1:      generateSHA1(source),
		CreatedAt: time.Now(),
	}

	le.scripts[name] = script
	return nil
}

func (le *LuaEngine) ExecuteScript(name string, keys []string, args []string) (*ScriptResult, error) {
	le.mu.RLock()
	script, exists := le.scripts[name]
	le.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("script not found: %s", name)
	}

	// Create new state for execution
	L := lua.NewState()
	defer L.Close()

	// Register functions
	le.registerFunctions()

	// Set up keys and arguments
	keysTable := L.NewTable()
	for i, key := range keys {
		keysTable.RawSetInt(i+1, lua.LString(key))
	}
	L.SetGlobal("KEYS", keysTable)

	argsTable := L.NewTable()
	for i, arg := range args {
		argsTable.RawSetInt(i+1, lua.LString(arg))
	}
	L.SetGlobal("ARGV", argsTable)

	// Execute script
	if err := L.DoString(script.Source); err != nil {
		return &ScriptResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Get result from stack
	result := L.Get(-1)
	L.Pop(1)

	return &ScriptResult{
		Success: true,
		Result:  luaValueToInterface(result),
	}, nil
}

func (le *LuaEngine) ExecuteSource(source string, keys []string, args []string) (*ScriptResult, error) {
	// Create new state for execution
	L := lua.NewState()
	defer L.Close()

	// Register functions
	le.registerFunctions()

	// Set up keys and arguments
	keysTable := L.NewTable()
	for i, key := range keys {
		keysTable.RawSetInt(i+1, lua.LString(key))
	}
	L.SetGlobal("KEYS", keysTable)

	argsTable := L.NewTable()
	for i, arg := range args {
		argsTable.RawSetInt(i+1, lua.LString(arg))
	}
	L.SetGlobal("ARGV", argsTable)

	// Execute script
	if err := L.DoString(source); err != nil {
		return &ScriptResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Get result from stack
	result := L.Get(-1)
	L.Pop(1)

	return &ScriptResult{
		Success: true,
		Result:  luaValueToInterface(result),
	}, nil
}

func (le *LuaEngine) ListScripts() []*LuaScript {
	le.mu.RLock()
	defer le.mu.RUnlock()

	var scripts []*LuaScript
	for _, script := range le.scripts {
		scripts = append(scripts, script)
	}
	return scripts
}

func (le *LuaEngine) GetScript(name string) (*LuaScript, bool) {
	le.mu.RLock()
	defer le.mu.RUnlock()

	script, exists := le.scripts[name]
	return script, exists
}

func (le *LuaEngine) DeleteScript(name string) bool {
	le.mu.Lock()
	defer le.mu.Unlock()

	if _, exists := le.scripts[name]; exists {
		delete(le.scripts, name)
		return true
	}
	return false
}

func (le *LuaEngine) FlushScripts() {
	le.mu.Lock()
	defer le.mu.Unlock()

	le.scripts = make(map[string]*LuaScript)
}

// Helper functions
func luaValueToInterface(value lua.LValue) interface{} {
	switch v := value.(type) {
	case lua.LString:
		return string(v)
	case lua.LNumber:
		return float64(v)
	case lua.LBool:
		return bool(v)
	case *lua.LTable:
		return tableToMap(v)
	case lua.LNilType:
		return nil
	default:
		return v.String()
	}
}

func tableToMap(table *lua.LTable) map[string]interface{} {
	result := make(map[string]interface{})
	table.ForEach(func(key, value lua.LValue) {
		keyStr := key.String()
		result[keyStr] = luaValueToInterface(value)
	})
	return result
}

func generateSHA1(data string) string {
	// Simple hash implementation (in production, use crypto/sha1)
	hash := 0
	for _, char := range data {
		hash = ((hash << 5) - hash) + int(char)
		hash = hash & hash // Convert to 32-bit integer
	}
	return fmt.Sprintf("%x", hash)
}

// Predefined financial scripts
func (le *LuaEngine) LoadFinancialScripts() error {
	scripts := map[string]string{
		"calculate_vwap": `
			local total_volume = 0
			local total_value = 0
			
			for i = 1, #KEYS do
				local price_key = KEYS[i] .. ":price"
				local volume_key = KEYS[i] .. ":volume"
				
				local price = tonumber(redis.get(price_key)) or 0
				local volume = tonumber(redis.get(volume_key)) or 0
				
				total_value = total_value + (price * volume)
				total_volume = total_volume + volume
			end
			
			if total_volume > 0 then
				return total_value / total_volume
			else
				return 0
			end
		`,

		"fraud_detection": `
			local user_id = ARGV[1]
			local amount = tonumber(ARGV[2])
			local merchant = ARGV[3]
			
			-- Get user's transaction history
			local txn_count = tonumber(redis.get(user_id .. ":txn_count:1h")) or 0
			local total_amount = tonumber(redis.get(user_id .. ":total_amount:1h")) or 0
			local fraud_score = tonumber(redis.get(user_id .. ":fraud_score")) or 0
			
			-- Calculate risk factors
			local velocity_risk = 0
			if txn_count > 10 then
				velocity_risk = (txn_count - 10) * 0.1
			end
			
			local amount_risk = 0
			if amount > 1000 then
				amount_risk = (amount - 1000) * 0.001
			end
			
			local new_fraud_score = fraud_score + velocity_risk + amount_risk
			
			-- Update counters
			redis.set(user_id .. ":txn_count:1h", txn_count + 1)
			redis.set(user_id .. ":total_amount:1h", total_amount + amount)
			redis.set(user_id .. ":fraud_score", new_fraud_score)
			
			-- Return risk assessment
			if new_fraud_score > 0.8 then
				return "HIGH_RISK"
			elseif new_fraud_score > 0.5 then
				return "MEDIUM_RISK"
			else
				return "LOW_RISK"
			end
		`,

		"order_matching": `
			local symbol = ARGV[1]
			local order_id = ARGV[2]
			local side = ARGV[3]
			local price = tonumber(ARGV[4])
			local quantity = tonumber(ARGV[5])
			
			local orderbook_key = "orderbook:" .. symbol
			local matched_orders = {}
			
			if side == "BUY" then
				-- Look for matching sell orders
				local asks = redis.zrange(orderbook_key, 0, -1)
				for i, ask in ipairs(asks) do
					local ask_price = tonumber(redis.zscore(orderbook_key, ask))
					if ask_price <= price then
						table.insert(matched_orders, ask)
					end
				end
			else
				-- Look for matching buy orders
				local bids = redis.zrevrange(orderbook_key, 0, -1)
				for i, bid in ipairs(bids) do
					local bid_price = tonumber(redis.zscore(orderbook_key, bid))
					if bid_price >= price then
						table.insert(matched_orders, bid)
					end
				end
			end
			
			return matched_orders
		`,

		"portfolio_value": `
			local portfolio_id = ARGV[1]
			local total_value = 0
			
			-- Get portfolio positions
			local positions = redis.zrange(portfolio_id .. ":positions", 0, -1)
			
			for i, position in ipairs(positions) do
				local quantity = tonumber(redis.zscore(portfolio_id .. ":positions", position))
				local price_key = "price:" .. position
				local current_price = tonumber(redis.get(price_key)) or 0
				
				total_value = total_value + (quantity * current_price)
			end
			
			return total_value
		`,
	}

	for name, source := range scripts {
		if err := le.LoadScript(name, source); err != nil {
			return fmt.Errorf("failed to load script %s: %v", name, err)
		}
	}

	return nil
}
