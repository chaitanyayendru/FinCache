package store

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
)

type HyperLogLog struct {
	mu        sync.RWMutex
	registers []uint8
	precision int
	m         int // 2^precision
	alpha     float64
}

type HyperLogLogResult struct {
	Cardinality uint64  `json:"cardinality"`
	Estimate    float64 `json:"estimate"`
	Error       float64 `json:"error"`
	Registers   int     `json:"registers"`
}

func NewHyperLogLog(precision int) (*HyperLogLog, error) {
	if precision < 4 || precision > 16 {
		return nil, fmt.Errorf("precision must be between 4 and 16, got %d", precision)
	}

	m := 1 << precision
	alpha := getAlpha(m)

	return &HyperLogLog{
		registers: make([]uint8, m),
		precision: precision,
		m:         m,
		alpha:     alpha,
	}, nil
}

func (hll *HyperLogLog) Add(element string) {
	hll.mu.Lock()
	defer hll.mu.Unlock()

	hash := hll.hash(element)
	index := hash & uint64(hll.m-1)
	leadingZeros := hll.countLeadingZeros(hash >> hll.precision)

	if hll.registers[index] < leadingZeros {
		hll.registers[index] = leadingZeros
	}
}

func (hll *HyperLogLog) Count() uint64 {
	hll.mu.RLock()
	defer hll.mu.RUnlock()

	sum := 0.0
	zeroCount := 0

	for _, register := range hll.registers {
		sum += math.Pow(2, -float64(register))
		if register == 0 {
			zeroCount++
		}
	}

	estimate := hll.alpha * float64(hll.m*hll.m) / sum

	// Apply bias correction for small cardinalities
	if estimate <= 2.5*float64(hll.m) {
		if zeroCount > 0 {
			estimate = float64(hll.m) * math.Log(float64(hll.m)/float64(zeroCount))
		}
	}

	// Apply bias correction for very large cardinalities
	if estimate > 1.0/30.0*math.Pow(2, 32) {
		estimate = -math.Pow(2, 32) * math.Log(1-estimate/math.Pow(2, 32))
	}

	return uint64(estimate)
}

func (hll *HyperLogLog) Merge(other *HyperLogLog) error {
	hll.mu.Lock()
	defer hll.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	if hll.precision != other.precision {
		return fmt.Errorf("cannot merge HyperLogLog with different precision: %d != %d", hll.precision, other.precision)
	}

	for i := 0; i < hll.m; i++ {
		if other.registers[i] > hll.registers[i] {
			hll.registers[i] = other.registers[i]
		}
	}

	return nil
}

func (hll *HyperLogLog) GetResult() *HyperLogLogResult {
	cardinality := hll.Count()
	estimate := float64(cardinality)
	error := hll.getStandardError()

	return &HyperLogLogResult{
		Cardinality: cardinality,
		Estimate:    estimate,
		Error:       error,
		Registers:   hll.m,
	}
}

func (hll *HyperLogLog) Reset() {
	hll.mu.Lock()
	defer hll.mu.Unlock()

	for i := range hll.registers {
		hll.registers[i] = 0
	}
}

func (hll *HyperLogLog) GetStats() map[string]interface{} {
	hll.mu.RLock()
	defer hll.mu.RUnlock()

	stats := map[string]interface{}{
		"precision":   hll.precision,
		"registers":   hll.m,
		"alpha":       hll.alpha,
		"cardinality": hll.Count(),
	}

	// Calculate register statistics
	maxRegister := uint8(0)
	minRegister := uint8(255)
	zeroCount := 0
	sum := 0

	for _, register := range hll.registers {
		if register > maxRegister {
			maxRegister = register
		}
		if register < minRegister {
			minRegister = register
		}
		if register == 0 {
			zeroCount++
		}
		sum += int(register)
	}

	stats["max_register"] = maxRegister
	stats["min_register"] = minRegister
	stats["zero_registers"] = zeroCount
	stats["avg_register"] = float64(sum) / float64(hll.m)

	return stats
}

// Financial-specific HyperLogLog methods
func (hll *HyperLogLog) AddTransaction(txID string) {
	hll.Add(txID)
}

func (hll *HyperLogLog) AddUser(userID string) {
	hll.Add(userID)
}

func (hll *HyperLogLog) AddMerchant(merchantID string) {
	hll.Add(merchantID)
}

func (hll *HyperLogLog) AddIPAddress(ipAddress string) {
	hll.Add(ipAddress)
}

func (hll *HyperLogLog) AddDevice(deviceID string) {
	hll.Add(deviceID)
}

func (hll *HyperLogLog) AddCard(cardHash string) {
	hll.Add(cardHash)
}

// Fraud detection specific methods
func (hll *HyperLogLog) GetUniqueTransactions() uint64 {
	return hll.Count()
}

func (hll *HyperLogLog) GetUniqueUsers() uint64 {
	return hll.Count()
}

func (hll *HyperLogLog) GetUniqueMerchants() uint64 {
	return hll.Count()
}

func (hll *HyperLogLog) GetUniqueIPs() uint64 {
	return hll.Count()
}

func (hll *HyperLogLog) GetUniqueDevices() uint64 {
	return hll.Count()
}

func (hll *HyperLogLog) GetUniqueCards() uint64 {
	return hll.Count()
}

// Helper methods
func (hll *HyperLogLog) hash(element string) uint64 {
	hash := md5.Sum([]byte(element))
	return binary.BigEndian.Uint64(hash[:8])
}

func (hll *HyperLogLog) countLeadingZeros(value uint64) uint8 {
	if value == 0 {
		return 64
	}

	count := uint8(0)
	for value&0x8000000000000000 == 0 {
		count++
		value <<= 1
	}
	return count
}

func getAlpha(m int) float64 {
	switch m {
	case 16:
		return 0.673
	case 32:
		return 0.697
	case 64:
		return 0.709
	default:
		return 0.7213 / (1 + 1.079/float64(m))
	}
}

func (hll *HyperLogLog) getStandardError() float64 {
	return 1.04 / math.Sqrt(float64(hll.m))
}

// HyperLogLog store for managing multiple HLL instances
type HyperLogLogStore struct {
	mu        sync.RWMutex
	instances map[string]*HyperLogLog
}

func NewHyperLogLogStore() *HyperLogLogStore {
	return &HyperLogLogStore{
		instances: make(map[string]*HyperLogLog),
	}
}

func (hlls *HyperLogLogStore) Create(key string, precision int) error {
	hlls.mu.Lock()
	defer hlls.mu.Unlock()

	if _, exists := hlls.instances[key]; exists {
		return fmt.Errorf("HyperLogLog already exists: %s", key)
	}

	hll, err := NewHyperLogLog(precision)
	if err != nil {
		return err
	}

	hlls.instances[key] = hll
	return nil
}

func (hlls *HyperLogLogStore) Add(key, element string) error {
	hlls.mu.RLock()
	hll, exists := hlls.instances[key]
	hlls.mu.RUnlock()

	if !exists {
		return fmt.Errorf("HyperLogLog not found: %s", key)
	}

	hll.Add(element)
	return nil
}

func (hlls *HyperLogLogStore) Count(key string) (uint64, error) {
	hlls.mu.RLock()
	hll, exists := hlls.instances[key]
	hlls.mu.RUnlock()

	if !exists {
		return 0, fmt.Errorf("HyperLogLog not found: %s", key)
	}

	return hll.Count(), nil
}

func (hlls *HyperLogLogStore) Merge(targetKey, sourceKey string) error {
	hlls.mu.RLock()
	target, exists := hlls.instances[targetKey]
	if !exists {
		hlls.mu.RUnlock()
		return fmt.Errorf("target HyperLogLog not found: %s", targetKey)
	}

	source, exists := hlls.instances[sourceKey]
	if !exists {
		hlls.mu.RUnlock()
		return fmt.Errorf("source HyperLogLog not found: %s", sourceKey)
	}
	hlls.mu.RUnlock()

	return target.Merge(source)
}

func (hlls *HyperLogLogStore) Delete(key string) error {
	hlls.mu.Lock()
	defer hlls.mu.Unlock()

	if _, exists := hlls.instances[key]; !exists {
		return fmt.Errorf("HyperLogLog not found: %s", key)
	}

	delete(hlls.instances, key)
	return nil
}

func (hlls *HyperLogLogStore) GetStats(key string) (map[string]interface{}, error) {
	hlls.mu.RLock()
	hll, exists := hlls.instances[key]
	hlls.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("HyperLogLog not found: %s", key)
	}

	return hll.GetStats(), nil
}

func (hlls *HyperLogLogStore) GetAllStats() map[string]interface{} {
	hlls.mu.RLock()
	defer hlls.mu.RUnlock()

	stats := map[string]interface{}{
		"total_instances": len(hlls.instances),
		"instances":       make(map[string]interface{}),
	}

	for key, hll := range hlls.instances {
		stats["instances"].(map[string]interface{})[key] = hll.GetStats()
	}

	return stats
}

// Financial analytics methods
func (hlls *HyperLogLogStore) TrackDailyTransactions(date string) error {
	key := fmt.Sprintf("daily_transactions:%s", date)
	if err := hlls.Create(key, 12); err != nil && err.Error() != "HyperLogLog already exists: "+key {
		return err
	}
	return nil
}

func (hlls *HyperLogLogStore) TrackHourlyTransactions(date, hour string) error {
	key := fmt.Sprintf("hourly_transactions:%s:%s", date, hour)
	if err := hlls.Create(key, 12); err != nil && err.Error() != "HyperLogLog already exists: "+key {
		return err
	}
	return nil
}

func (hlls *HyperLogLogStore) TrackUserActivity(userID, period string) error {
	key := fmt.Sprintf("user_activity:%s:%s", userID, period)
	if err := hlls.Create(key, 10); err != nil && err.Error() != "HyperLogLog already exists: "+key {
		return err
	}
	return nil
}

func (hlls *HyperLogLogStore) TrackMerchantActivity(merchantID, period string) error {
	key := fmt.Sprintf("merchant_activity:%s:%s", merchantID, period)
	if err := hlls.Create(key, 10); err != nil && err.Error() != "HyperLogLog already exists: "+key {
		return err
	}
	return nil
}

func (hlls *HyperLogLogStore) GetDailyTransactionCount(date string) (uint64, error) {
	key := fmt.Sprintf("daily_transactions:%s", date)
	return hlls.Count(key)
}

func (hlls *HyperLogLogStore) GetHourlyTransactionCount(date, hour string) (uint64, error) {
	key := fmt.Sprintf("hourly_transactions:%s:%s", date, hour)
	return hlls.Count(key)
}

func (hlls *HyperLogLogStore) GetUserActivityCount(userID, period string) (uint64, error) {
	key := fmt.Sprintf("user_activity:%s:%s", userID, period)
	return hlls.Count(key)
}

func (hlls *HyperLogLogStore) GetMerchantActivityCount(merchantID, period string) (uint64, error) {
	key := fmt.Sprintf("merchant_activity:%s:%s", merchantID, period)
	return hlls.Count(key)
}
