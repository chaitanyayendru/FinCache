package store

import (
	"fmt"
	"math"
	"sync"
)

type GeoPoint struct {
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	Name      string  `json:"name"`
	Distance  float64 `json:"distance,omitempty"`
}

type GeoStore struct {
	mu     sync.RWMutex
	points map[string]*GeoPoint
	index  map[string]map[string]bool // region -> point names
}

type GeoRadiusResult struct {
	Points []*GeoPoint `json:"points"`
	Count  int         `json:"count"`
	Radius float64     `json:"radius"`
	Unit   string      `json:"unit"`
}

type GeoSearchResult struct {
	Points []*GeoPoint `json:"points"`
	Count  int         `json:"count"`
	Box    [4]float64  `json:"box"` // [min_lon, min_lat, max_lon, max_lat]
}

func NewGeoStore() *GeoStore {
	return &GeoStore{
		points: make(map[string]*GeoPoint),
		index:  make(map[string]map[string]bool),
	}
}

func (gs *GeoStore) GeoAdd(key string, longitude, latitude float64, name string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// Validate coordinates
	if longitude < -180 || longitude > 180 {
		return fmt.Errorf("invalid longitude: %f", longitude)
	}
	if latitude < -90 || latitude > 90 {
		return fmt.Errorf("invalid latitude: %f", latitude)
	}

	point := &GeoPoint{
		Longitude: longitude,
		Latitude:  latitude,
		Name:      name,
	}

	gs.points[name] = point

	// Add to spatial index
	region := gs.getRegion(longitude, latitude)
	if gs.index[region] == nil {
		gs.index[region] = make(map[string]bool)
	}
	gs.index[region][name] = true

	return nil
}

func (gs *GeoStore) GeoRemove(key string, name string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	point, exists := gs.points[name]
	if !exists {
		return fmt.Errorf("point not found: %s", name)
	}

	// Remove from spatial index
	region := gs.getRegion(point.Longitude, point.Latitude)
	if gs.index[region] != nil {
		delete(gs.index[region], name)
		if len(gs.index[region]) == 0 {
			delete(gs.index, region)
		}
	}

	// Remove point
	delete(gs.points, name)

	return nil
}

func (gs *GeoStore) GeoPos(key string, name string) (*GeoPoint, error) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	point, exists := gs.points[name]
	if !exists {
		return nil, fmt.Errorf("point not found: %s", name)
	}

	return point, nil
}

func (gs *GeoStore) GeoDist(key string, name1, name2 string, unit string) (float64, error) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	point1, exists := gs.points[name1]
	if !exists {
		return 0, fmt.Errorf("point not found: %s", name1)
	}

	point2, exists := gs.points[name2]
	if !exists {
		return 0, fmt.Errorf("point not found: %s", name2)
	}

	distance := gs.calculateDistance(point1, point2)

	// Convert to requested unit
	switch unit {
	case "m", "meters":
		return distance * 1000, nil
	case "km", "kilometers":
		return distance, nil
	case "mi", "miles":
		return distance * 0.621371, nil
	case "ft", "feet":
		return distance * 3280.84, nil
	default:
		return distance, nil // Default to kilometers
	}
}

func (gs *GeoStore) GeoRadius(key string, longitude, latitude, radius float64, unit string) (*GeoRadiusResult, error) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	// Convert radius to kilometers
	radiusKm := gs.convertRadiusToKm(radius, unit)

	var results []*GeoPoint
	center := &GeoPoint{Longitude: longitude, Latitude: latitude}

	for name, point := range gs.points {
		distance := gs.calculateDistance(center, point)
		if distance <= radiusKm {
			pointCopy := *point
			pointCopy.Distance = distance
			results = append(results, &pointCopy)
		}
	}

	return &GeoRadiusResult{
		Points: results,
		Count:  len(results),
		Radius: radius,
		Unit:   unit,
	}, nil
}

func (gs *GeoStore) GeoRadiusByMember(key string, member string, radius float64, unit string) (*GeoRadiusResult, error) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	center, exists := gs.points[member]
	if !exists {
		return nil, fmt.Errorf("member not found: %s", member)
	}

	// Convert radius to kilometers
	radiusKm := gs.convertRadiusToKm(radius, unit)

	var results []*GeoPoint

	for name, point := range gs.points {
		distance := gs.calculateDistance(center, point)
		if distance <= radiusKm {
			pointCopy := *point
			pointCopy.Distance = distance
			results = append(results, &pointCopy)
		}
	}

	return &GeoRadiusResult{
		Points: results,
		Count:  len(results),
		Radius: radius,
		Unit:   unit,
	}, nil
}

func (gs *GeoStore) GeoSearch(key string, longitude, latitude, width, height float64) (*GeoSearchResult, error) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	minLon := longitude - width/2
	maxLon := longitude + width/2
	minLat := latitude - height/2
	maxLat := latitude + height/2

	var results []*GeoPoint

	for _, point := range gs.points {
		if point.Longitude >= minLon && point.Longitude <= maxLon &&
			point.Latitude >= minLat && point.Latitude <= maxLat {
			results = append(results, point)
		}
	}

	return &GeoSearchResult{
		Points: results,
		Count:  len(results),
		Box:    [4]float64{minLon, minLat, maxLon, maxLat},
	}, nil
}

func (gs *GeoStore) GeoHash(key string, name string) (string, error) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	point, exists := gs.points[name]
	if !exists {
		return "", fmt.Errorf("point not found: %s", name)
	}

	// Simple geohash implementation
	// In production, use a proper geohash library
	return gs.encodeGeohash(point.Longitude, point.Latitude), nil
}

// Financial-specific geospatial methods
func (gs *GeoStore) AddATM(key string, atmID string, longitude, latitude float64, bank string) error {
	name := fmt.Sprintf("atm:%s", atmID)
	return gs.GeoAdd(key, longitude, latitude, name)
}

func (gs *GeoStore) AddMerchant(key string, merchantID string, longitude, latitude float64, category string) error {
	name := fmt.Sprintf("merchant:%s", merchantID)
	return gs.GeoAdd(key, longitude, latitude, name)
}

func (gs *GeoStore) AddUserLocation(key string, userID string, longitude, latitude float64, timestamp int64) error {
	name := fmt.Sprintf("user:%s:%d", userID, timestamp)
	return gs.GeoAdd(key, longitude, latitude, name)
}

func (gs *GeoStore) FindNearbyATMs(key string, longitude, latitude, radius float64) (*GeoRadiusResult, error) {
	return gs.GeoRadius(key, longitude, latitude, radius, "km")
}

func (gs *GeoStore) FindNearbyMerchants(key string, longitude, latitude, radius float64, category string) (*GeoRadiusResult, error) {
	result, err := gs.GeoRadius(key, longitude, latitude, radius, "km")
	if err != nil {
		return nil, err
	}

	// Filter by category if specified
	if category != "" {
		var filtered []*GeoPoint
		for _, point := range result.Points {
			if gs.getMerchantCategory(point.Name) == category {
				filtered = append(filtered, point)
			}
		}
		result.Points = filtered
		result.Count = len(filtered)
	}

	return result, nil
}

func (gs *GeoStore) DetectLocationAnomaly(key string, userID string, longitude, latitude float64, maxDistance float64) (bool, error) {
	// Get user's recent locations
	recentLocations := gs.getUserRecentLocations(key, userID, 5)

	if len(recentLocations) == 0 {
		return false, nil // No previous locations to compare
	}

	// Calculate distance to most recent location
	latestLocation := recentLocations[len(recentLocations)-1]
	distance := gs.calculateDistance(&GeoPoint{Longitude: longitude, Latitude: latitude}, latestLocation)

	return distance > maxDistance, nil
}

func (gs *GeoStore) GetTravelDistance(key string, userID string, startTime, endTime int64) (float64, error) {
	locations := gs.getUserLocationsInRange(key, userID, startTime, endTime)

	if len(locations) < 2 {
		return 0, nil
	}

	totalDistance := 0.0
	for i := 1; i < len(locations); i++ {
		distance := gs.calculateDistance(locations[i-1], locations[i])
		totalDistance += distance
	}

	return totalDistance, nil
}

// Helper methods
func (gs *GeoStore) calculateDistance(p1, p2 *GeoPoint) float64 {
	// Haversine formula for calculating distance between two points
	const R = 6371 // Earth's radius in kilometers

	lat1 := p1.Latitude * math.Pi / 180
	lat2 := p2.Latitude * math.Pi / 180
	deltaLat := (p2.Latitude - p1.Latitude) * math.Pi / 180
	deltaLon := (p2.Longitude - p1.Longitude) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func (gs *GeoStore) convertRadiusToKm(radius float64, unit string) float64 {
	switch unit {
	case "m", "meters":
		return radius / 1000
	case "km", "kilometers":
		return radius
	case "mi", "miles":
		return radius * 1.60934
	case "ft", "feet":
		return radius * 0.0003048
	default:
		return radius // Default to kilometers
	}
}

func (gs *GeoStore) getRegion(longitude, latitude float64) string {
	// Simple region calculation for spatial indexing
	// In production, use a more sophisticated spatial index like R-tree
	lonRegion := int(longitude / 10)
	latRegion := int(latitude / 10)
	return fmt.Sprintf("%d,%d", lonRegion, latRegion)
}

func (gs *GeoStore) encodeGeohash(longitude, latitude float64) string {
	// Simple geohash implementation
	// In production, use a proper geohash library
	lonBits := int((longitude + 180) * 65536 / 360)
	latBits := int((latitude + 90) * 32768 / 180)

	combined := (uint64(lonBits) << 32) | uint64(latBits)

	const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"
	var hash string

	for i := 0; i < 12; i++ {
		hash = string(base32[combined&31]) + hash
		combined >>= 5
	}

	return hash
}

func (gs *GeoStore) getMerchantCategory(name string) string {
	// Extract category from merchant name
	// In production, store category as metadata
	return "general"
}

func (gs *GeoStore) getUserRecentLocations(key string, userID string, count int) []*GeoPoint {
	var locations []*GeoPoint

	for name, point := range gs.points {
		if gs.isUserLocation(name, userID) {
			locations = append(locations, point)
		}
	}

	// Sort by timestamp (extracted from name)
	// In production, use proper timestamp sorting
	return locations
}

func (gs *GeoStore) getUserLocationsInRange(key string, userID string, startTime, endTime int64) []*GeoPoint {
	var locations []*GeoPoint

	for name, point := range gs.points {
		if gs.isUserLocationInRange(name, userID, startTime, endTime) {
			locations = append(locations, point)
		}
	}

	return locations
}

func (gs *GeoStore) isUserLocation(name, userID string) bool {
	return len(name) > len("user:")+len(userID)+1 &&
		name[:len("user:")+len(userID)] == "user:"+userID
}

func (gs *GeoStore) isUserLocationInRange(name, userID string, startTime, endTime int64) bool {
	if !gs.isUserLocation(name, userID) {
		return false
	}

	// Extract timestamp from name
	// In production, use proper timestamp parsing
	return true
}

func (gs *GeoStore) GetStats() map[string]interface{} {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	stats := map[string]interface{}{
		"total_points":  len(gs.points),
		"total_regions": len(gs.index),
		"regions":       make(map[string]int),
	}

	for region, points := range gs.index {
		stats["regions"].(map[string]interface{})[region] = len(points)
	}

	return stats
}
