package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

type ClusterNode struct {
	ID        string            `json:"id"`
	Address   string            `json:"address"`
	Port      int               `json:"port"`
	Role      NodeRole          `json:"role"`
	State     NodeState         `json:"state"`
	Slots     []int             `json:"slots"`
	Flags     map[string]bool   `json:"flags"`
	PingSent  time.Time         `json:"ping_sent"`
	PongRecv  time.Time         `json:"pong_recv"`
	Epoch     int64             `json:"epoch"`
	Connected bool              `json:"connected"`
	Metadata  map[string]string `json:"metadata"`
}

type NodeRole string

const (
	RoleMaster NodeRole = "master"
	RoleSlave  NodeRole = "slave"
)

type NodeState string

const (
	StateConnected    NodeState = "connected"
	StateDisconnected NodeState = "disconnected"
	StateFail         NodeState = "fail"
	StatePfail        NodeState = "pfail"
)

type ClusterManager struct {
	mu              sync.RWMutex
	nodes           map[string]*ClusterNode
	self            *ClusterNode
	slots           map[int]*ClusterNode
	config          ClusterConfig
	logger          *zap.Logger
	ctx             context.Context
	cancel          context.CancelFunc
	heartbeatTicker *time.Ticker
}

type ClusterConfig struct {
	NodeID      string
	Address     string
	Port        int
	Slots       []int
	Replicas    int
	HeartbeatMs int
	TimeoutMs   int
}

type ClusterInfo struct {
	State         string            `json:"state"`
	SlotsAssigned int               `json:"slots_assigned"`
	SlotsOK       int               `json:"slots_ok"`
	SlotsPFail    int               `json:"slots_pfail"`
	SlotsFail     int               `json:"slots_fail"`
	KnownNodes    int               `json:"known_nodes"`
	Size          int               `json:"size"`
	CurrentEpoch  int64             `json:"current_epoch"`
	MyEpoch       int64             `json:"my_epoch"`
	Stats         map[string]string `json:"stats"`
}

func NewClusterManager(config ClusterConfig, logger *zap.Logger) *ClusterManager {
	ctx, cancel := context.WithCancel(context.Background())

	cm := &ClusterManager{
		nodes:  make(map[string]*ClusterNode),
		slots:  make(map[int]*ClusterNode),
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize self node
	cm.self = &ClusterNode{
		ID:        config.NodeID,
		Address:   config.Address,
		Port:      config.Port,
		Role:      RoleMaster,
		State:     StateConnected,
		Slots:     config.Slots,
		Flags:     make(map[string]bool),
		Connected: true,
		Metadata:  make(map[string]string),
		Epoch:     time.Now().UnixNano(),
	}

	cm.nodes[config.NodeID] = cm.self

	// Assign slots to self
	for _, slot := range config.Slots {
		cm.slots[slot] = cm.self
	}

	// Start heartbeat
	cm.startHeartbeat()

	return cm
}

func (cm *ClusterManager) startHeartbeat() {
	interval := time.Duration(cm.config.HeartbeatMs) * time.Millisecond
	cm.heartbeatTicker = time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-cm.ctx.Done():
				return
			case <-cm.heartbeatTicker.C:
				cm.sendHeartbeat()
			}
		}
	}()
}

func (cm *ClusterManager) sendHeartbeat() {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	now := time.Now()
	cm.self.PingSent = now

	// Send PING to all other nodes
	for nodeID, node := range cm.nodes {
		if nodeID == cm.self.ID {
			continue
		}

		// In a real implementation, this would send actual network messages
		cm.logger.Debug("Sending heartbeat",
			zap.String("to_node", nodeID),
			zap.String("from_node", cm.self.ID))
	}
}

func (cm *ClusterManager) AddNode(nodeID, address string, port int, slots []int) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.nodes[nodeID]; exists {
		return fmt.Errorf("node already exists: %s", nodeID)
	}

	node := &ClusterNode{
		ID:        nodeID,
		Address:   address,
		Port:      port,
		Role:      RoleMaster,
		State:     StateConnected,
		Slots:     slots,
		Flags:     make(map[string]bool),
		Connected: true,
		Metadata:  make(map[string]string),
		Epoch:     time.Now().UnixNano(),
	}

	cm.nodes[nodeID] = node

	// Assign slots to node
	for _, slot := range slots {
		cm.slots[slot] = node
	}

	cm.logger.Info("Node added to cluster",
		zap.String("node_id", nodeID),
		zap.String("address", address),
		zap.Int("port", port),
		zap.Ints("slots", slots))

	return nil
}

func (cm *ClusterManager) RemoveNode(nodeID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	node, exists := cm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	// Remove slots assigned to this node
	for _, slot := range node.Slots {
		delete(cm.slots, slot)
	}

	// Remove node
	delete(cm.nodes, nodeID)

	cm.logger.Info("Node removed from cluster",
		zap.String("node_id", nodeID))

	return nil
}

func (cm *ClusterManager) GetNode(nodeID string) (*ClusterNode, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	node, exists := cm.nodes[nodeID]
	return node, exists
}

func (cm *ClusterManager) ListNodes() []*ClusterNode {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var nodes []*ClusterNode
	for _, node := range cm.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (cm *ClusterManager) GetNodeForSlot(slot int) (*ClusterNode, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	node, exists := cm.slots[slot]
	return node, exists
}

func (cm *ClusterManager) GetNodeForKey(key string) (*ClusterNode, bool) {
	slot := cm.HashSlot(key)
	return cm.GetNodeForSlot(slot)
}

func (cm *ClusterManager) HashSlot(key string) int {
	// Simple hash slot implementation
	// In production, use CRC16 or similar
	hash := 0
	for _, char := range key {
		hash = ((hash << 5) - hash) + int(char)
		hash = hash & hash // Convert to 32-bit integer
	}
	return hash % 16384 // Redis uses 16384 slots
}

func (cm *ClusterManager) GetClusterInfo() *ClusterInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	info := &ClusterInfo{
		State:        "ok",
		KnownNodes:   len(cm.nodes),
		Size:         len(cm.nodes),
		CurrentEpoch: cm.self.Epoch,
		MyEpoch:      cm.self.Epoch,
		Stats:        make(map[string]string),
	}

	// Count slots
	for _, node := range cm.nodes {
		info.SlotsAssigned += len(node.Slots)
		if node.State == StateConnected {
			info.SlotsOK += len(node.Slots)
		} else if node.State == StatePfail {
			info.SlotsPFail += len(node.Slots)
		} else if node.State == StateFail {
			info.SlotsFail += len(node.Slots)
		}
	}

	// Add stats
	info.Stats["messages_sent"] = "0"
	info.Stats["messages_received"] = "0"
	info.Stats["keyspace_hits"] = "0"
	info.Stats["keyspace_misses"] = "0"

	return info
}

func (cm *ClusterManager) SetNodeState(nodeID string, state NodeState) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	node, exists := cm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	oldState := node.State
	node.State = state
	node.Connected = (state == StateConnected)

	cm.logger.Info("Node state changed",
		zap.String("node_id", nodeID),
		zap.String("old_state", string(oldState)),
		zap.String("new_state", string(state)))

	return nil
}

func (cm *ClusterManager) UpdateNodeMetadata(nodeID string, metadata map[string]string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	node, exists := cm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	for key, value := range metadata {
		node.Metadata[key] = value
	}

	return nil
}

func (cm *ClusterManager) RebalanceSlots() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Simple rebalancing: distribute slots evenly among master nodes
	var masterNodes []*ClusterNode
	for _, node := range cm.nodes {
		if node.Role == RoleMaster {
			masterNodes = append(masterNodes, node)
		}
	}

	if len(masterNodes) == 0 {
		return fmt.Errorf("no master nodes available")
	}

	// Clear all slot assignments
	for slot := range cm.slots {
		delete(cm.slots, slot)
	}

	// Redistribute slots
	slotsPerNode := 16384 / len(masterNodes)
	extraSlots := 16384 % len(masterNodes)

	slotIndex := 0
	for i, node := range masterNodes {
		// Clear node's slots
		node.Slots = []int{}

		// Assign slots to this node
		slotsToAssign := slotsPerNode
		if i < extraSlots {
			slotsToAssign++
		}

		for j := 0; j < slotsToAssign; j++ {
			cm.slots[slotIndex] = node
			node.Slots = append(node.Slots, slotIndex)
			slotIndex++
		}
	}

	cm.logger.Info("Cluster slots rebalanced",
		zap.Int("total_slots", 16384),
		zap.Int("master_nodes", len(masterNodes)))

	return nil
}

func (cm *ClusterManager) Failover(nodeID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	node, exists := cm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	if node.Role != RoleMaster {
		return fmt.Errorf("can only failover master nodes")
	}

	// Find a slave to promote
	var slaveNode *ClusterNode
	for _, n := range cm.nodes {
		if n.Role == RoleSlave && n.State == StateConnected {
			slaveNode = n
			break
		}
	}

	if slaveNode == nil {
		return fmt.Errorf("no available slave nodes for failover")
	}

	// Promote slave to master
	slaveNode.Role = RoleMaster
	slaveNode.Slots = node.Slots
	slaveNode.Epoch = time.Now().UnixNano()

	// Update slot assignments
	for _, slot := range node.Slots {
		cm.slots[slot] = slaveNode
	}

	// Mark old master as failed
	node.State = StateFail
	node.Connected = false

	cm.logger.Info("Failover completed",
		zap.String("old_master", nodeID),
		zap.String("new_master", slaveNode.ID))

	return nil
}

func (cm *ClusterManager) AddReplica(masterID, replicaID, replicaAddress string, replicaPort int) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	master, exists := cm.nodes[masterID]
	if !exists {
		return fmt.Errorf("master node not found: %s", masterID)
	}

	if master.Role != RoleMaster {
		return fmt.Errorf("node is not a master: %s", masterID)
	}

	replica := &ClusterNode{
		ID:        replicaID,
		Address:   replicaAddress,
		Port:      replicaPort,
		Role:      RoleSlave,
		State:     StateConnected,
		Slots:     []int{}, // Replicas don't own slots
		Flags:     make(map[string]bool),
		Connected: true,
		Metadata:  make(map[string]string),
		Epoch:     time.Now().UnixNano(),
	}

	cm.nodes[replicaID] = replica

	cm.logger.Info("Replica added",
		zap.String("master_id", masterID),
		zap.String("replica_id", replicaID),
		zap.String("replica_address", replicaAddress))

	return nil
}

func (cm *ClusterManager) Close() error {
	cm.cancel()
	if cm.heartbeatTicker != nil {
		cm.heartbeatTicker.Stop()
	}
	return nil
}

// Cluster-aware routing
func (cm *ClusterManager) RouteCommand(key string) (*ClusterNode, error) {
	node, exists := cm.GetNodeForKey(key)
	if !exists {
		return nil, fmt.Errorf("no node available for key: %s", key)
	}

	if node.State != StateConnected {
		return nil, fmt.Errorf("node %s is not connected", node.ID)
	}

	return node, nil
}

// Cluster health check
func (cm *ClusterManager) HealthCheck() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	health := map[string]interface{}{
		"status":    "healthy",
		"nodes":     len(cm.nodes),
		"connected": 0,
		"failed":    0,
		"slots":     len(cm.slots),
	}

	for _, node := range cm.nodes {
		if node.Connected {
			health["connected"] = health["connected"].(int) + 1
		} else {
			health["failed"] = health["failed"].(int) + 1
		}
	}

	if health["failed"].(int) > 0 {
		health["status"] = "degraded"
	}

	return health
}
