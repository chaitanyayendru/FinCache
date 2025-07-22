package store

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

type JSONStore struct {
	mu        sync.RWMutex
	documents map[string]*JSONDocument
	indexes   map[string]*JSONIndex
}

type JSONDocument struct {
	ID       string                 `json:"id"`
	Data     map[string]interface{} `json:"data"`
	Created  int64                  `json:"created"`
	Modified int64                  `json:"modified"`
	TTL      *int64                 `json:"ttl,omitempty"`
}

type JSONIndex struct {
	mu        sync.RWMutex
	fields    map[string]*IndexField
	documents map[string]bool
}

type IndexField struct {
	FieldName string
	Type      string
	Values    map[interface{}][]string // value -> document IDs
}

type JSONQuery struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

type JSONQueryResult struct {
	Documents []*JSONDocument `json:"documents"`
	Total     int             `json:"total"`
	Limit     int             `json:"limit"`
	Offset    int             `json:"offset"`
}

func NewJSONStore() *JSONStore {
	return &JSONStore{
		documents: make(map[string]*JSONDocument),
		indexes:   make(map[string]*JSONIndex),
	}
}

func (js *JSONStore) Set(key string, data interface{}, ttl *int64) error {
	js.mu.Lock()
	defer js.mu.Unlock()

	// Convert data to map[string]interface{}
	dataMap, err := js.convertToMap(data)
	if err != nil {
		return fmt.Errorf("failed to convert data: %v", err)
	}

	now := time.Now().Unix()

	doc := &JSONDocument{
		ID:       key,
		Data:     dataMap,
		Created:  now,
		Modified: now,
		TTL:      ttl,
	}

	js.documents[key] = doc

	// Update indexes
	js.updateIndexes(key, dataMap)

	return nil
}

func (js *JSONStore) Get(key string) (*JSONDocument, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()

	doc, exists := js.documents[key]
	if !exists {
		return nil, fmt.Errorf("document not found: %s", key)
	}

	// Check TTL
	if doc.TTL != nil && time.Now().Unix() > *doc.TTL {
		return nil, fmt.Errorf("document expired: %s", key)
	}

	return doc, nil
}

func (js *JSONStore) Delete(key string) error {
	js.mu.Lock()
	defer js.mu.Unlock()

	if _, exists := js.documents[key]; !exists {
		return fmt.Errorf("document not found: %s", key)
	}

	// Remove from indexes
	js.removeFromIndexes(key)

	// Remove document
	delete(js.documents, key)

	return nil
}

func (js *JSONStore) Query(queries []JSONQuery, limit, offset int) (*JSONQueryResult, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()

	var results []*JSONDocument

	// If no queries, return all documents
	if len(queries) == 0 {
		for _, doc := range js.documents {
			if doc.TTL == nil || time.Now().Unix() <= *doc.TTL {
				results = append(results, doc)
			}
		}
	} else {
		// Find documents matching all queries
		matchingDocs := js.findMatchingDocuments(queries)
		for docID := range matchingDocs {
			if doc, exists := js.documents[docID]; exists {
				if doc.TTL == nil || time.Now().Unix() <= *doc.TTL {
					results = append(results, doc)
				}
			}
		}
	}

	// Apply pagination
	total := len(results)
	if offset >= total {
		results = []*JSONDocument{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		results = results[offset:end]
	}

	return &JSONQueryResult{
		Documents: results,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func (js *JSONStore) findMatchingDocuments(queries []JSONQuery) map[string]bool {
	matchingDocs := make(map[string]bool)
	firstQuery := true

	for _, query := range queries {
		queryMatches := js.findDocumentsForQuery(query)

		if firstQuery {
			matchingDocs = queryMatches
			firstQuery = false
		} else {
			// Intersect with previous results
			for docID := range matchingDocs {
				if !queryMatches[docID] {
					delete(matchingDocs, docID)
				}
			}
		}
	}

	return matchingDocs
}

func (js *JSONStore) findDocumentsForQuery(query JSONQuery) map[string]bool {
	matches := make(map[string]bool)

	// Check if we have an index for this field
	if index, exists := js.indexes[query.Field]; exists {
		index.mu.RLock()
		if field, exists := index.fields[query.Field]; exists {
			if docIDs, exists := field.Values[query.Value]; exists {
				for _, docID := range docIDs {
					matches[docID] = true
				}
			}
		}
		index.mu.RUnlock()
	} else {
		// Fallback to scanning all documents
		for docID, doc := range js.documents {
			if js.documentMatchesQuery(doc, query) {
				matches[docID] = true
			}
		}
	}

	return matches
}

func (js *JSONStore) documentMatchesQuery(doc *JSONDocument, query JSONQuery) bool {
	value := js.getNestedValue(doc.Data, query.Field)

	switch query.Operator {
	case "=":
		return reflect.DeepEqual(value, query.Value)
	case "!=":
		return !reflect.DeepEqual(value, query.Value)
	case ">":
		return js.compareValues(value, query.Value) > 0
	case ">=":
		return js.compareValues(value, query.Value) >= 0
	case "<":
		return js.compareValues(value, query.Value) < 0
	case "<=":
		return js.compareValues(value, query.Value) <= 0
	case "contains":
		return js.containsValue(value, query.Value)
	case "starts_with":
		return js.startsWithValue(value, query.Value)
	case "ends_with":
		return js.endsWithValue(value, query.Value)
	default:
		return false
	}
}

func (js *JSONStore) getNestedValue(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			return current[part]
		}

		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}

	return nil
}

func (js *JSONStore) compareValues(a, b interface{}) int {
	// Simple comparison - in production, use proper type-aware comparison
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)

	if aStr < bStr {
		return -1
	} else if aStr > bStr {
		return 1
	}
	return 0
}

func (js *JSONStore) containsValue(value, search interface{}) bool {
	valueStr := fmt.Sprintf("%v", value)
	searchStr := fmt.Sprintf("%v", search)
	return strings.Contains(valueStr, searchStr)
}

func (js *JSONStore) startsWithValue(value, search interface{}) bool {
	valueStr := fmt.Sprintf("%v", value)
	searchStr := fmt.Sprintf("%v", search)
	return strings.HasPrefix(valueStr, searchStr)
}

func (js *JSONStore) endsWithValue(value, search interface{}) bool {
	valueStr := fmt.Sprintf("%v", value)
	searchStr := fmt.Sprintf("%v", search)
	return strings.HasSuffix(valueStr, searchStr)
}

func (js *JSONStore) CreateIndex(fieldName string) error {
	js.mu.Lock()
	defer js.mu.Unlock()

	if _, exists := js.indexes[fieldName]; exists {
		return fmt.Errorf("index already exists: %s", fieldName)
	}

	index := &JSONIndex{
		fields:    make(map[string]*IndexField),
		documents: make(map[string]bool),
	}

	index.fields[fieldName] = &IndexField{
		FieldName: fieldName,
		Type:      "string", // Default type
		Values:    make(map[interface{}][]string),
	}

	js.indexes[fieldName] = index

	// Build index from existing documents
	for docID, doc := range js.documents {
		js.addToIndex(fieldName, docID, doc.Data)
	}

	return nil
}

func (js *JSONStore) updateIndexes(docID string, data map[string]interface{}) {
	for fieldName := range js.indexes {
		js.addToIndex(fieldName, docID, data)
	}
}

func (js *JSONStore) addToIndex(fieldName, docID string, data map[string]interface{}) {
	if index, exists := js.indexes[fieldName]; exists {
		index.mu.Lock()
		defer index.mu.Unlock()

		if field, exists := index.fields[fieldName]; exists {
			value := js.getNestedValue(data, fieldName)
			if value != nil {
				if docIDs, exists := field.Values[value]; exists {
					field.Values[value] = append(docIDs, docID)
				} else {
					field.Values[value] = []string{docID}
				}
			}
		}
	}
}

func (js *JSONStore) removeFromIndexes(docID string) {
	for _, index := range js.indexes {
		index.mu.Lock()
		for _, field := range index.fields {
			for value, docIDs := range field.Values {
				var newDocIDs []string
				for _, id := range docIDs {
					if id != docID {
						newDocIDs = append(newDocIDs, id)
					}
				}
				if len(newDocIDs) == 0 {
					delete(field.Values, value)
				} else {
					field.Values[value] = newDocIDs
				}
			}
		}
		index.mu.Unlock()
	}
}

func (js *JSONStore) convertToMap(data interface{}) (map[string]interface{}, error) {
	switch v := data.(type) {
	case map[string]interface{}:
		return v, nil
	case string:
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(v), &result); err != nil {
			return nil, err
		}
		return result, nil
	case []byte:
		var result map[string]interface{}
		if err := json.Unmarshal(v, &result); err != nil {
			return nil, err
		}
		return result, nil
	default:
		// Try to marshal and unmarshal to convert to map
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}

		var result map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
}

func (js *JSONStore) GetStats() map[string]interface{} {
	js.mu.RLock()
	defer js.mu.RUnlock()

	stats := map[string]interface{}{
		"total_documents": len(js.documents),
		"total_indexes":   len(js.indexes),
		"indexes":         make(map[string]interface{}),
	}

	for fieldName, index := range js.indexes {
		index.mu.RLock()
		indexStats := map[string]interface{}{
			"fields": len(index.fields),
		}

		for fieldName, field := range index.fields {
			indexStats[fieldName] = map[string]interface{}{
				"type":   field.Type,
				"values": len(field.Values),
			}
		}
		index.mu.RUnlock()

		stats["indexes"].(map[string]interface{})[fieldName] = indexStats
	}

	return stats
}

// Financial-specific JSON methods
func (js *JSONStore) StoreTransaction(txID string, transaction map[string]interface{}) error {
	// Add metadata for financial transactions
	transaction["_type"] = "transaction"
	transaction["_timestamp"] = time.Now().Unix()

	return js.Set(txID, transaction, nil)
}

func (js *JSONStore) StoreUserProfile(userID string, profile map[string]interface{}) error {
	// Add metadata for user profiles
	profile["_type"] = "user_profile"
	profile["_timestamp"] = time.Now().Unix()

	return js.Set(userID, profile, nil)
}

func (js *JSONStore) StoreMarketData(symbol string, data map[string]interface{}) error {
	// Add metadata for market data
	data["_type"] = "market_data"
	data["_timestamp"] = time.Now().Unix()
	data["_symbol"] = symbol

	return js.Set(fmt.Sprintf("market:%s:%d", symbol, time.Now().Unix()), data, nil)
}

func (js *JSONStore) QueryTransactions(userID string, startTime, endTime int64) (*JSONQueryResult, error) {
	queries := []JSONQuery{
		{Field: "_type", Operator: "=", Value: "transaction"},
		{Field: "user_id", Operator: "=", Value: userID},
		{Field: "_timestamp", Operator: ">=", Value: startTime},
		{Field: "_timestamp", Operator: "<=", Value: endTime},
	}

	return js.Query(queries, 100, 0)
}

func (js *JSONStore) QueryUserProfiles(criteria map[string]interface{}) (*JSONQueryResult, error) {
	queries := []JSONQuery{
		{Field: "_type", Operator: "=", Value: "user_profile"},
	}

	for field, value := range criteria {
		queries = append(queries, JSONQuery{
			Field:    field,
			Operator: "=",
			Value:    value,
		})
	}

	return js.Query(queries, 50, 0)
}

func (js *JSONStore) QueryMarketData(symbol string, limit int) (*JSONQueryResult, error) {
	queries := []JSONQuery{
		{Field: "_type", Operator: "=", Value: "market_data"},
		{Field: "_symbol", Operator: "=", Value: symbol},
	}

	return js.Query(queries, limit, 0)
}
