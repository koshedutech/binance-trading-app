// Package autopilot provides instance control for active/standby coordination
// Epic 9, Story 9.6, Phase 3: Active/Standby Instance Control
package autopilot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis key constants for instance control
const (
	// KeyGinieActive stores the ID of the currently active instance
	KeyGinieActive = "ginie:active"

	// KeyGinieHeartbeat is the prefix for heartbeat keys (with TTL)
	// Full key: ginie:heartbeat:{instanceID}
	KeyGinieHeartbeat = "ginie:heartbeat:%s"

	// KeyGinieControl is the Pub/Sub channel for control signals
	KeyGinieControl = "ginie:control"

	// HeartbeatTTL is the TTL for heartbeat keys
	HeartbeatTTL = 30 * time.Second

	// HeartbeatInterval is how often heartbeats are sent
	HeartbeatInterval = 5 * time.Second

	// GracefulShutdownTimeout is how long to wait for ops to complete
	GracefulShutdownTimeout = 5 * time.Second
)

// ControlSignalType defines the types of control signals
type ControlSignalType string

const (
	// SignalDeactivate requests an instance to deactivate
	SignalDeactivate ControlSignalType = "DEACTIVATE"

	// SignalReady indicates an instance is ready (deactivated and safe)
	SignalReady ControlSignalType = "READY"

	// SignalActivate indicates an instance is becoming active
	SignalActivate ControlSignalType = "ACTIVATE"
)

// ControlSignal is the message format for Pub/Sub communication
type ControlSignal struct {
	Type         ControlSignalType `json:"type"`
	FromInstance string            `json:"from_instance"`
	ToInstance   string            `json:"to_instance"` // or "*" for broadcast
	Timestamp    int64             `json:"timestamp"`
}

// InstanceControl manages active/standby state coordination between instances
type InstanceControl struct {
	redis           *redis.Client
	instanceID      string
	isActive        atomic.Bool
	activeByDefault bool
	mu              sync.RWMutex
	onDeactivate    func()
	onActivate      func()
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup

	// Channel to receive READY signals when taking control
	readyChan chan string

	// Track last heartbeat time for status reporting
	lastHeartbeat atomic.Value // stores time.Time
}

// NewInstanceControl creates a new InstanceControl
// instanceID should come from INSTANCE_ID env var ("dev" or "prod")
// activeByDefault should come from ACTIVE_BY_DEFAULT env var
func NewInstanceControl(redisClient *redis.Client, instanceID string, activeByDefault bool) *InstanceControl {
	if instanceID == "" {
		instanceID = os.Getenv("INSTANCE_ID")
		if instanceID == "" {
			instanceID = "unknown"
		}
	}

	ic := &InstanceControl{
		redis:           redisClient,
		instanceID:      instanceID,
		activeByDefault: activeByDefault,
		readyChan:       make(chan string, 10),
	}

	log.Printf("[INSTANCE-CTRL] Created instance control: id=%s, activeByDefault=%v",
		instanceID, activeByDefault)

	return ic
}

// Start initializes the instance control system
// - Checks/claims active status based on Redis state
// - Starts heartbeat goroutine
// - Subscribes to control channel
func (ic *InstanceControl) Start(ctx context.Context) error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// Create cancellable context
	ic.ctx, ic.cancel = context.WithCancel(ctx)

	// Check current active instance in Redis
	activeInstance, err := ic.getActiveInstanceInternal(ic.ctx)
	if err != nil && err != redis.Nil {
		log.Printf("[INSTANCE-CTRL] Error checking active instance: %v", err)
		// Continue anyway - we'll try to establish state
	}

	log.Printf("[INSTANCE-CTRL] Current active instance in Redis: %q", activeInstance)

	// Determine if we should be active
	shouldBeActive := false

	if activeInstance == "" {
		// No active instance - use activeByDefault
		if ic.activeByDefault {
			// Try to claim active status with SETNX
			success, err := ic.redis.SetNX(ic.ctx, KeyGinieActive, ic.instanceID, 0).Result()
			if err != nil {
				log.Printf("[INSTANCE-CTRL] Error claiming active status: %v", err)
			} else if success {
				shouldBeActive = true
				log.Printf("[INSTANCE-CTRL] Successfully claimed active status (no previous active)")
			} else {
				// Someone else claimed it between our check and claim
				activeInstance, _ = ic.getActiveInstanceInternal(ic.ctx)
				log.Printf("[INSTANCE-CTRL] Another instance claimed active: %s", activeInstance)
			}
		}
	} else if activeInstance == ic.instanceID {
		// We were the active instance
		shouldBeActive = true
		log.Printf("[INSTANCE-CTRL] Resuming as active instance")
	} else {
		// Another instance is active - check if it's alive
		if !ic.isOtherInstanceAliveInternal(ic.ctx, activeInstance) {
			log.Printf("[INSTANCE-CTRL] Active instance %s appears dead, checking if we should take over", activeInstance)
			if ic.activeByDefault {
				// Take over
				if err := ic.redis.Set(ic.ctx, KeyGinieActive, ic.instanceID, 0).Err(); err != nil {
					log.Printf("[INSTANCE-CTRL] Error taking over active status: %v", err)
				} else {
					shouldBeActive = true
					log.Printf("[INSTANCE-CTRL] Took over active status from dead instance %s", activeInstance)
				}
			}
		} else {
			log.Printf("[INSTANCE-CTRL] Instance %s is active and alive, we'll be standby", activeInstance)
		}
	}

	// Set our active state
	ic.isActive.Store(shouldBeActive)

	if shouldBeActive {
		log.Printf("[INSTANCE-CTRL] Starting as ACTIVE instance")
		if ic.onActivate != nil {
			go ic.onActivate()
		}
	} else {
		log.Printf("[INSTANCE-CTRL] Starting as STANDBY instance")
	}

	// Start heartbeat goroutine
	ic.wg.Add(1)
	go ic.heartbeatLoop()

	// Start Pub/Sub listener
	ic.wg.Add(1)
	go ic.subscribeLoop()

	return nil
}

// Stop gracefully shuts down the instance control
func (ic *InstanceControl) Stop() {
	log.Printf("[INSTANCE-CTRL] Stopping instance control")

	ic.mu.Lock()
	if ic.cancel != nil {
		ic.cancel()
	}
	ic.mu.Unlock()

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		ic.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("[INSTANCE-CTRL] Gracefully stopped")
	case <-time.After(10 * time.Second):
		log.Printf("[INSTANCE-CTRL] Shutdown timed out")
	}
}

// IsActive returns whether this instance is currently active
func (ic *InstanceControl) IsActive() bool {
	return ic.isActive.Load()
}

// GetInstanceID returns this instance's ID
func (ic *InstanceControl) GetInstanceID() string {
	return ic.instanceID
}

// TakeControl initiates a graceful handover to become the active instance
// 1. Sends DEACTIVATE signal to current active
// 2. Waits for READY response
// 3. Claims active status
func (ic *InstanceControl) TakeControl(ctx context.Context) error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	if ic.isActive.Load() {
		log.Printf("[INSTANCE-CTRL] Already active, nothing to do")
		return nil
	}

	// Get current active instance
	activeInstance, err := ic.getActiveInstanceInternal(ctx)
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get active instance: %w", err)
	}

	if activeInstance == "" || activeInstance == ic.instanceID {
		// No active instance or we're somehow marked as active
		ic.becomeActive(ctx)
		return nil
	}

	// Check if the other instance is alive
	if !ic.isOtherInstanceAliveInternal(ctx, activeInstance) {
		log.Printf("[INSTANCE-CTRL] Active instance %s is dead, taking over immediately", activeInstance)
		ic.becomeActive(ctx)
		return nil
	}

	log.Printf("[INSTANCE-CTRL] Requesting control from %s", activeInstance)

	// Clear ready channel
	select {
	case <-ic.readyChan:
	default:
	}

	// Send DEACTIVATE signal
	signal := ControlSignal{
		Type:         SignalDeactivate,
		FromInstance: ic.instanceID,
		ToInstance:   activeInstance,
		Timestamp:    time.Now().Unix(),
	}

	signalData, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("failed to marshal deactivate signal: %w", err)
	}

	if err := ic.redis.Publish(ctx, KeyGinieControl, signalData).Err(); err != nil {
		return fmt.Errorf("failed to publish deactivate signal: %w", err)
	}

	log.Printf("[INSTANCE-CTRL] Sent DEACTIVATE to %s, waiting for READY", activeInstance)

	// Wait for READY signal with timeout
	select {
	case readyFrom := <-ic.readyChan:
		if readyFrom == activeInstance {
			log.Printf("[INSTANCE-CTRL] Received READY from %s", activeInstance)
		} else {
			log.Printf("[INSTANCE-CTRL] Received READY from unexpected instance %s (expected %s)", readyFrom, activeInstance)
		}
	case <-time.After(GracefulShutdownTimeout + 2*time.Second):
		log.Printf("[INSTANCE-CTRL] Timeout waiting for READY from %s, taking control anyway", activeInstance)
	case <-ctx.Done():
		return ctx.Err()
	}

	// Become active
	ic.becomeActive(ctx)
	return nil
}

// ReleaseControl voluntarily releases active status
func (ic *InstanceControl) ReleaseControl(ctx context.Context) error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	if !ic.isActive.Load() {
		log.Printf("[INSTANCE-CTRL] Not active, nothing to release")
		return nil
	}

	log.Printf("[INSTANCE-CTRL] Voluntarily releasing control")

	// Deactivate first
	ic.deactivate(ctx)

	// Clear active status in Redis if it's us
	activeInstance, _ := ic.getActiveInstanceInternal(ctx)
	if activeInstance == ic.instanceID {
		if err := ic.redis.Del(ctx, KeyGinieActive).Err(); err != nil {
			log.Printf("[INSTANCE-CTRL] Error clearing active status: %v", err)
		}
	}

	// Send READY signal to broadcast
	signal := ControlSignal{
		Type:         SignalReady,
		FromInstance: ic.instanceID,
		ToInstance:   "*",
		Timestamp:    time.Now().Unix(),
	}

	signalData, _ := json.Marshal(signal)
	ic.redis.Publish(ctx, KeyGinieControl, signalData)

	return nil
}

// SetCallbacks sets the activation and deactivation callbacks
func (ic *InstanceControl) SetCallbacks(onActivate, onDeactivate func()) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	ic.onActivate = onActivate
	ic.onDeactivate = onDeactivate
}

// GetActiveInstance returns the ID of the currently active instance
// Returns empty string if no active instance or on error
func (ic *InstanceControl) GetActiveInstance(ctx context.Context) string {
	result, _ := ic.getActiveInstanceInternal(ctx)
	return result
}

// GetLastHeartbeat returns the time of the last successful heartbeat
func (ic *InstanceControl) GetLastHeartbeat() time.Time {
	val := ic.lastHeartbeat.Load()
	if val == nil {
		return time.Time{}
	}
	return val.(time.Time)
}

// ForceTakeControl immediately takes control without waiting for graceful handover
// Use when the other instance is unresponsive or in emergency situations
func (ic *InstanceControl) ForceTakeControl(ctx context.Context) error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	if ic.isActive.Load() {
		log.Printf("[INSTANCE-CTRL] Already active, nothing to do")
		return nil
	}

	log.Printf("[INSTANCE-CTRL] Force taking control")

	// Directly become active without waiting for READY
	ic.becomeActive(ctx)
	return nil
}

// IsOtherInstanceAlive checks if another instance is alive via heartbeat
func (ic *InstanceControl) IsOtherInstanceAlive(ctx context.Context) bool {
	activeInstance, err := ic.getActiveInstanceInternal(ctx)
	if err != nil {
		return false
	}

	// If another instance is active, check if it's alive
	if activeInstance != "" && activeInstance != ic.instanceID {
		return ic.isOtherInstanceAliveInternal(ctx, activeInstance)
	}

	// If WE are active (or no one is active), check if the OTHER instance has a heartbeat
	// Determine the "other" instance ID based on our ID
	var otherInstanceID string
	switch ic.instanceID {
	case "prod":
		otherInstanceID = "dev"
	case "dev":
		otherInstanceID = "prod"
	default:
		// Unknown instance ID - check for any heartbeat that's not ours
		return ic.checkAnyOtherHeartbeat(ctx)
	}

	return ic.isOtherInstanceAliveInternal(ctx, otherInstanceID)
}

// Internal methods

func (ic *InstanceControl) getActiveInstanceInternal(ctx context.Context) (string, error) {
	result, err := ic.redis.Get(ctx, KeyGinieActive).Result()
	if err == redis.Nil {
		return "", nil
	}
	return result, err
}

func (ic *InstanceControl) isOtherInstanceAliveInternal(ctx context.Context, instanceID string) bool {
	heartbeatKey := fmt.Sprintf(KeyGinieHeartbeat, instanceID)
	exists, err := ic.redis.Exists(ctx, heartbeatKey).Result()
	if err != nil {
		log.Printf("[INSTANCE-CTRL] Error checking heartbeat for %s: %v", instanceID, err)
		return false
	}
	return exists > 0
}

// checkAnyOtherHeartbeat checks if any other instance has a heartbeat
// This is used as a fallback when the instance ID is not dev or prod
func (ic *InstanceControl) checkAnyOtherHeartbeat(ctx context.Context) bool {
	// Scan for all heartbeat keys
	pattern := "ginie:heartbeat:*"
	keys, err := ic.redis.Keys(ctx, pattern).Result()
	if err != nil {
		log.Printf("[INSTANCE-CTRL] Error scanning heartbeat keys: %v", err)
		return false
	}

	ourHeartbeatKey := fmt.Sprintf(KeyGinieHeartbeat, ic.instanceID)
	for _, key := range keys {
		if key != ourHeartbeatKey {
			// Found another instance's heartbeat
			exists, err := ic.redis.Exists(ctx, key).Result()
			if err == nil && exists > 0 {
				return true
			}
		}
	}
	return false
}

func (ic *InstanceControl) becomeActive(ctx context.Context) {
	// Set active in Redis
	if err := ic.redis.Set(ctx, KeyGinieActive, ic.instanceID, 0).Err(); err != nil {
		log.Printf("[INSTANCE-CTRL] Error setting active status: %v", err)
	}

	ic.isActive.Store(true)
	log.Printf("[INSTANCE-CTRL] Now ACTIVE")

	// Send ACTIVATE signal
	signal := ControlSignal{
		Type:         SignalActivate,
		FromInstance: ic.instanceID,
		ToInstance:   "*",
		Timestamp:    time.Now().Unix(),
	}

	signalData, _ := json.Marshal(signal)
	ic.redis.Publish(ctx, KeyGinieControl, signalData)

	// Call activation callback
	if ic.onActivate != nil {
		go ic.onActivate()
	}
}

func (ic *InstanceControl) deactivate(ctx context.Context) {
	wasActive := ic.isActive.Swap(false)

	if wasActive {
		log.Printf("[INSTANCE-CTRL] Deactivating - waiting for current operations")

		// Call deactivation callback and wait for completion
		if ic.onDeactivate != nil {
			done := make(chan struct{})
			go func() {
				ic.onDeactivate()
				close(done)
			}()

			select {
			case <-done:
				log.Printf("[INSTANCE-CTRL] Deactivation callback completed")
			case <-time.After(GracefulShutdownTimeout):
				log.Printf("[INSTANCE-CTRL] Deactivation callback timed out")
			}
		}

		log.Printf("[INSTANCE-CTRL] Now STANDBY")
	}
}

func (ic *InstanceControl) heartbeatLoop() {
	defer ic.wg.Done()

	heartbeatKey := fmt.Sprintf(KeyGinieHeartbeat, ic.instanceID)
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	// Send initial heartbeat
	ic.sendHeartbeat(heartbeatKey)

	for {
		select {
		case <-ic.ctx.Done():
			log.Printf("[INSTANCE-CTRL] Heartbeat loop stopping")
			// Clear heartbeat on shutdown
			ic.redis.Del(context.Background(), heartbeatKey)
			return
		case <-ticker.C:
			ic.sendHeartbeat(heartbeatKey)
		}
	}
}

func (ic *InstanceControl) sendHeartbeat(key string) {
	now := time.Now()
	timestamp := now.Unix()
	if err := ic.redis.Set(ic.ctx, key, timestamp, HeartbeatTTL).Err(); err != nil {
		log.Printf("[INSTANCE-CTRL] Error sending heartbeat: %v", err)
	} else {
		// Track successful heartbeat time
		ic.lastHeartbeat.Store(now)
	}
}

func (ic *InstanceControl) subscribeLoop() {
	defer ic.wg.Done()

	pubsub := ic.redis.Subscribe(ic.ctx, KeyGinieControl)
	defer pubsub.Close()

	log.Printf("[INSTANCE-CTRL] Subscribed to control channel: %s", KeyGinieControl)

	ch := pubsub.Channel()

	for {
		select {
		case <-ic.ctx.Done():
			log.Printf("[INSTANCE-CTRL] Pub/Sub loop stopping")
			return
		case msg, ok := <-ch:
			if !ok {
				log.Printf("[INSTANCE-CTRL] Pub/Sub channel closed")
				return
			}
			ic.handleControlSignal(msg.Payload)
		}
	}
}

func (ic *InstanceControl) handleControlSignal(payload string) {
	var signal ControlSignal
	if err := json.Unmarshal([]byte(payload), &signal); err != nil {
		log.Printf("[INSTANCE-CTRL] Error parsing control signal: %v", err)
		return
	}

	// Ignore our own signals
	if signal.FromInstance == ic.instanceID {
		return
	}

	// Check if signal is for us
	if signal.ToInstance != "*" && signal.ToInstance != ic.instanceID {
		return
	}

	log.Printf("[INSTANCE-CTRL] Received %s from %s", signal.Type, signal.FromInstance)

	switch signal.Type {
	case SignalDeactivate:
		ic.handleDeactivateSignal(signal)

	case SignalReady:
		// Forward to TakeControl if waiting
		select {
		case ic.readyChan <- signal.FromInstance:
		default:
			log.Printf("[INSTANCE-CTRL] READY signal received but no one waiting")
		}

	case SignalActivate:
		// Another instance is becoming active
		if ic.isActive.Load() {
			log.Printf("[INSTANCE-CTRL] Another instance (%s) activated, we should deactivate",
				signal.FromInstance)
			ic.mu.Lock()
			ic.deactivate(ic.ctx)
			ic.mu.Unlock()
		}
	}
}

func (ic *InstanceControl) handleDeactivateSignal(signal ControlSignal) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	if !ic.isActive.Load() {
		log.Printf("[INSTANCE-CTRL] Received DEACTIVATE but already standby")
		// Still send READY
	} else {
		log.Printf("[INSTANCE-CTRL] Deactivating per request from %s", signal.FromInstance)
		ic.deactivate(ic.ctx)
	}

	// Clear active status if it's us
	activeInstance, _ := ic.getActiveInstanceInternal(ic.ctx)
	if activeInstance == ic.instanceID {
		ic.redis.Del(ic.ctx, KeyGinieActive)
	}

	// Send READY signal
	readySignal := ControlSignal{
		Type:         SignalReady,
		FromInstance: ic.instanceID,
		ToInstance:   signal.FromInstance,
		Timestamp:    time.Now().Unix(),
	}

	signalData, _ := json.Marshal(readySignal)
	if err := ic.redis.Publish(ic.ctx, KeyGinieControl, signalData).Err(); err != nil {
		log.Printf("[INSTANCE-CTRL] Error sending READY signal: %v", err)
	} else {
		log.Printf("[INSTANCE-CTRL] Sent READY to %s", signal.FromInstance)
	}
}

// GetInstanceControlFromEnv creates an InstanceControl using environment variables
// Reads INSTANCE_ID and ACTIVE_BY_DEFAULT from environment
func GetInstanceControlFromEnv(redisClient *redis.Client) *InstanceControl {
	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = "unknown"
	}

	activeByDefault := false
	if val := os.Getenv("ACTIVE_BY_DEFAULT"); val == "true" || val == "1" {
		activeByDefault = true
	}

	return NewInstanceControl(redisClient, instanceID, activeByDefault)
}

// Status returns the current status of the instance control
type InstanceControlStatus struct {
	InstanceID      string `json:"instance_id"`
	IsActive        bool   `json:"is_active"`
	ActiveByDefault bool   `json:"active_by_default"`
	ActiveInstance  string `json:"active_instance"`
	OtherAlive      bool   `json:"other_instance_alive"`
}

// GetStatus returns the current status
func (ic *InstanceControl) GetStatus(ctx context.Context) InstanceControlStatus {
	return InstanceControlStatus{
		InstanceID:      ic.instanceID,
		IsActive:        ic.IsActive(),
		ActiveByDefault: ic.activeByDefault,
		ActiveInstance:  ic.GetActiveInstance(ctx),
		OtherAlive:      ic.IsOtherInstanceAlive(ctx),
	}
}
