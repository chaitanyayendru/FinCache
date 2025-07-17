package store

import (
	"sort"
	"sync"
)

type SortedSet struct {
	mu      sync.RWMutex
	members map[string]*SortedSetMember
	scores  map[float64]map[string]bool // score -> set of members
}

type SortedSetMember struct {
	Member string
	Score  float64
}

func NewSortedSet() *SortedSet {
	return &SortedSet{
		members: make(map[string]*SortedSetMember),
		scores:  make(map[float64]map[string]bool),
	}
}

func (ss *SortedSet) ZAdd(key string, score float64, member string) int {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	added := 0

	// Check if member already exists
	if existing, exists := ss.members[member]; exists {
		// Remove from old score
		if oldScoreMembers, exists := ss.scores[existing.Score]; exists {
			delete(oldScoreMembers, member)
			if len(oldScoreMembers) == 0 {
				delete(ss.scores, existing.Score)
			}
		}
	} else {
		added = 1
	}

	// Add to new score
	if _, exists := ss.scores[score]; !exists {
		ss.scores[score] = make(map[string]bool)
	}
	ss.scores[score][member] = true

	// Update member
	ss.members[member] = &SortedSetMember{
		Member: member,
		Score:  score,
	}

	return added
}

func (ss *SortedSet) ZRem(key string, members ...string) int {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	removed := 0

	for _, member := range members {
		if existing, exists := ss.members[member]; exists {
			// Remove from score
			if scoreMembers, exists := ss.scores[existing.Score]; exists {
				delete(scoreMembers, member)
				if len(scoreMembers) == 0 {
					delete(ss.scores, existing.Score)
				}
			}

			// Remove member
			delete(ss.members, member)
			removed++
		}
	}

	return removed
}

func (ss *SortedSet) ZScore(key string, member string) (float64, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	if existing, exists := ss.members[member]; exists {
		return existing.Score, true
	}
	return 0, false
}

func (ss *SortedSet) ZRank(key string, member string) int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	if _, exists := ss.members[member]; !exists {
		return -1
	}

	rank := 0
	for score := range ss.scores {
		if score < ss.members[member].Score {
			rank += len(ss.scores[score])
		}
	}
	return rank
}

func (ss *SortedSet) ZRevRank(key string, member string) int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	if _, exists := ss.members[member]; !exists {
		return -1
	}

	rank := 0
	for score := range ss.scores {
		if score > ss.members[member].Score {
			rank += len(ss.scores[score])
		}
	}
	return rank
}

func (ss *SortedSet) ZRange(key string, start, stop int) []string {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	return ss.getRange(key, start, stop, false)
}

func (ss *SortedSet) ZRevRange(key string, start, stop int) []string {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	return ss.getRange(key, start, stop, true)
}

func (ss *SortedSet) ZRangeWithScores(key string, start, stop int) []*SortedSetMember {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	return ss.getRangeWithScores(key, start, stop, false)
}

func (ss *SortedSet) ZRevRangeWithScores(key string, start, stop int) []*SortedSetMember {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	return ss.getRangeWithScores(key, start, stop, true)
}

func (ss *SortedSet) ZRangeByScore(key string, min, max float64) []string {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	var result []string
	for score := range ss.scores {
		if score >= min && score <= max {
			for member := range ss.scores[score] {
				result = append(result, member)
			}
		}
	}
	return result
}

func (ss *SortedSet) ZCount(key string, min, max float64) int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	count := 0
	for score := range ss.scores {
		if score >= min && score <= max {
			count += len(ss.scores[score])
		}
	}
	return count
}

func (ss *SortedSet) ZCard(key string) int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	return len(ss.members)
}

func (ss *SortedSet) ZIncrBy(key string, increment float64, member string) float64 {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	var newScore float64
	if existing, exists := ss.members[member]; exists {
		newScore = existing.Score + increment
	} else {
		newScore = increment
	}

	ss.ZAdd(key, newScore, member)
	return newScore
}

func (ss *SortedSet) ZRemRangeByRank(key string, start, stop int) int {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Get all members in order
	allMembers := ss.getAllMembersOrdered()

	removed := 0
	for i := start; i <= stop && i < len(allMembers); i++ {
		if ss.ZRem(key, allMembers[i].Member) > 0 {
			removed++
		}
	}

	return removed
}

func (ss *SortedSet) ZRemRangeByScore(key string, min, max float64) int {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	removed := 0
	var toRemove []string

	for score := range ss.scores {
		if score >= min && score <= max {
			for member := range ss.scores[score] {
				toRemove = append(toRemove, member)
			}
		}
	}

	for _, member := range toRemove {
		if ss.ZRem(key, member) > 0 {
			removed++
		}
	}

	return removed
}

func (ss *SortedSet) getRange(key string, start, stop int, reverse bool) []string {
	allMembers := ss.getAllMembersOrdered()
	if reverse {
		// Reverse the slice
		for i, j := 0, len(allMembers)-1; i < j; i, j = i+1, j-1 {
			allMembers[i], allMembers[j] = allMembers[j], allMembers[i]
		}
	}

	// Handle negative indices
	if start < 0 {
		start = len(allMembers) + start
	}
	if stop < 0 {
		stop = len(allMembers) + stop
	}

	// Bounds checking
	if start < 0 {
		start = 0
	}
	if stop >= len(allMembers) {
		stop = len(allMembers) - 1
	}
	if start > stop {
		return []string{}
	}

	var result []string
	for i := start; i <= stop && i < len(allMembers); i++ {
		result = append(result, allMembers[i].Member)
	}

	return result
}

func (ss *SortedSet) getRangeWithScores(key string, start, stop int, reverse bool) []*SortedSetMember {
	allMembers := ss.getAllMembersOrdered()
	if reverse {
		// Reverse the slice
		for i, j := 0, len(allMembers)-1; i < j; i, j = i+1, j-1 {
			allMembers[i], allMembers[j] = allMembers[j], allMembers[i]
		}
	}

	// Handle negative indices
	if start < 0 {
		start = len(allMembers) + start
	}
	if stop < 0 {
		stop = len(allMembers) + stop
	}

	// Bounds checking
	if start < 0 {
		start = 0
	}
	if stop >= len(allMembers) {
		stop = len(allMembers) - 1
	}
	if start > stop {
		return []*SortedSetMember{}
	}

	var result []*SortedSetMember
	for i := start; i <= stop && i < len(allMembers); i++ {
		result = append(result, allMembers[i])
	}

	return result
}

func (ss *SortedSet) getAllMembersOrdered() []*SortedSetMember {
	var scores []float64
	for score := range ss.scores {
		scores = append(scores, score)
	}
	sort.Float64s(scores)

	var result []*SortedSetMember
	for _, score := range scores {
		for member := range ss.scores[score] {
			result = append(result, &SortedSetMember{
				Member: member,
				Score:  score,
			})
		}
	}

	return result
}

// Order Book specific methods for financial applications
func (ss *SortedSet) GetOrderBook(depth int) ([]*SortedSetMember, []*SortedSetMember) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	// Get all members ordered by score
	allMembers := ss.getAllMembersOrdered()

	var bids, asks []*SortedSetMember

	for _, member := range allMembers {
		if member.Score > 0 { // Positive scores are bids
			if len(bids) < depth {
				bids = append(bids, member)
			}
		} else { // Negative scores are asks (we store them as negative for proper ordering)
			if len(asks) < depth {
				asks = append(asks, member)
			}
		}
	}

	// Reverse asks to get proper order (lowest ask first)
	for i, j := 0, len(asks)-1; i < j; i, j = i+1, j-1 {
		asks[i], asks[j] = asks[j], asks[i]
	}

	return bids, asks
}

func (ss *SortedSet) GetBestBid() (*SortedSetMember, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	var bestBid *SortedSetMember
	var bestScore float64

	for score := range ss.scores {
		if score > 0 && score > bestScore {
			bestScore = score
			for member := range ss.scores[score] {
				bestBid = &SortedSetMember{
					Member: member,
					Score:  score,
				}
				break // Take the first member at this score
			}
		}
	}

	return bestBid, bestBid != nil
}

func (ss *SortedSet) GetBestAsk() (*SortedSetMember, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	var bestAsk *SortedSetMember
	var bestScore float64

	for score := range ss.scores {
		if score < 0 && (bestAsk == nil || score > bestScore) {
			bestScore = score
			for member := range ss.scores[score] {
				bestAsk = &SortedSetMember{
					Member: member,
					Score:  score,
				}
				break // Take the first member at this score
			}
		}
	}

	return bestAsk, bestAsk != nil
}

func (ss *SortedSet) GetSpread() (float64, bool) {
	bestBid, bidExists := ss.GetBestBid()
	bestAsk, askExists := ss.GetBestAsk()

	if !bidExists || !askExists {
		return 0, false
	}

	// Since asks are stored as negative, we need to convert back
	spread := -bestAsk.Score - bestBid.Score
	return spread, true
}
