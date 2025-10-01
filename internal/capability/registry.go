package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/loqalabs/loqa-core/internal/bus"
	"github.com/loqalabs/loqa-core/internal/config"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Capability struct {
	Name       string            `json:"name"`
	Tier       string            `json:"tier,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type NodeInfo struct {
	ID           string       `json:"id"`
	Role         string       `json:"role"`
	Capabilities []Capability `json:"capabilities"`
	LastSeen     time.Time    `json:"last_seen"`
	Healthy      bool         `json:"healthy"`
}

type announceMessage struct {
	NodeID       string       `json:"node_id"`
	Role         string       `json:"role"`
	Capabilities []Capability `json:"capabilities"`
	Timestamp    time.Time    `json:"timestamp"`
}

type heartbeatMessage struct {
	NodeID    string    `json:"node_id"`
	Timestamp time.Time `json:"timestamp"`
}

type Registry struct {
	cfg       config.NodeConfig
	log       *slog.Logger
	bus       *bus.Client
	mu        sync.RWMutex
	nodes     map[string]*NodeInfo
	heartbeat *time.Ticker
	cancel    context.CancelFunc
	subs      []*nats.Subscription
	meter     metric.Meter
	nodeGauge metric.Int64ObservableGauge
	attrGauge metric.Int64ObservableGauge
}

func NewRegistry(ctx context.Context, cfg config.NodeConfig, busClient *bus.Client, log *slog.Logger) (*Registry, error) {
	ctx, cancel := context.WithCancel(ctx)
	r := &Registry{
		cfg:    cfg,
		log:    log.With(slog.String("component", "capability-registry")),
		bus:    busClient,
		nodes:  make(map[string]*NodeInfo),
		meter:  otel.Meter("github.com/loqalabs/loqa-core/runtime"),
		cancel: cancel,
	}

	if err := r.initMetrics(ctx); err != nil {
		r.log.Warn("failed to initialize metrics", slog.String("error", err.Error()))
	}

	if err := r.subscribe(ctx); err != nil {
		r.cancel()
		return nil, err
	}

	r.heartbeat = time.NewTicker(time.Duration(cfg.HeartbeatInterval) * time.Millisecond)
	go r.runHeartbeat(ctx)
	go r.monitorHealth(ctx)

	if err := r.announce(); err != nil {
		r.log.Warn("failed to announce node", slog.String("error", err.Error()))
	}

	return r, nil
}

func (r *Registry) Close() {
	if r.cancel != nil {
		r.cancel()
	}
	if r.heartbeat != nil {
		r.heartbeat.Stop()
	}
	for _, sub := range r.subs {
		_ = sub.Drain()
	}
}

func (r *Registry) subscribe(ctx context.Context) error {
	conn := r.bus.Conn()
	announceSub, err := conn.Subscribe("ctrl.node.announce", r.handleAnnounce)
	if err != nil {
		return fmt.Errorf("subscribe announce: %w", err)
	}
	r.subs = append(r.subs, announceSub)

	heartbeatSub, err := conn.Subscribe("ctrl.node.heartbeat.*", r.handleHeartbeat)
	if err != nil {
		return fmt.Errorf("subscribe heartbeat: %w", err)
	}
	r.subs = append(r.subs, heartbeatSub)

	return nil
}

func (r *Registry) runHeartbeat(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.heartbeat.C:
			if err := r.publishHeartbeat(); err != nil {
				r.log.Warn("failed to publish heartbeat", slog.String("error", err.Error()))
			}
		}
	}
}

func (r *Registry) monitorHealth(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.evaluateHealth()
		}
	}
}

func (r *Registry) announce() error {
	msg := announceMessage{
		NodeID:       r.cfg.ID,
		Role:         r.cfg.Role,
		Capabilities: convertCapabilities(r.cfg.Capabilities),
		Timestamp:    time.Now().UTC(),
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if err := r.bus.Conn().Publish("ctrl.node.announce", payload); err != nil {
		return err
	}
	r.updateNode(msg.NodeID, msg.Role, msg.Capabilities, msg.Timestamp, true)
	return nil
}

func (r *Registry) publishHeartbeat() error {
	msg := heartbeatMessage{
		NodeID:    r.cfg.ID,
		Timestamp: time.Now().UTC(),
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("ctrl.node.heartbeat.%s", r.cfg.ID)
	return r.bus.Conn().Publish(subject, payload)
}

func (r *Registry) handleAnnounce(msg *nats.Msg) {
	var announcement announceMessage
	if err := json.Unmarshal(msg.Data, &announcement); err != nil {
		r.log.Warn("invalid announce message", slog.String("error", err.Error()))
		return
	}
	if announcement.Timestamp.IsZero() {
		announcement.Timestamp = time.Now().UTC()
	}
	r.updateNode(announcement.NodeID, announcement.Role, announcement.Capabilities, announcement.Timestamp, true)
}

func (r *Registry) handleHeartbeat(msg *nats.Msg) {
	var hb heartbeatMessage
	if err := json.Unmarshal(msg.Data, &hb); err != nil {
		r.log.Warn("invalid heartbeat message", slog.String("error", err.Error()))
		return
	}
	if hb.Timestamp.IsZero() {
		hb.Timestamp = time.Now().UTC()
	}
	r.updateNode(hb.NodeID, "", nil, hb.Timestamp, true)
}

func (r *Registry) updateNode(nodeID, role string, capabilities []Capability, timestamp time.Time, healthy bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	node, ok := r.nodes[nodeID]
	if !ok {
		node = &NodeInfo{ID: nodeID}
		r.nodes[nodeID] = node
	}
	if role != "" {
		node.Role = role
	}
	if len(capabilities) > 0 {
		node.Capabilities = capabilities
	}
	node.LastSeen = timestamp
	node.Healthy = healthy
}

func (r *Registry) evaluateHealth() {
	r.mu.Lock()
	defer r.mu.Unlock()

	timeout := time.Duration(r.cfg.HeartbeatTimeout) * time.Millisecond
	now := time.Now()
	for _, node := range r.nodes {
		if now.Sub(node.LastSeen) > timeout {
			node.Healthy = false
		}
	}
}

func (r *Registry) Healthy() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	node, ok := r.nodes[r.cfg.ID]
	if !ok {
		return false
	}
	return node.Healthy
}

func (r *Registry) Query(filter func(NodeInfo) bool) []NodeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []NodeInfo
	for _, node := range r.nodes {
		copy := *node
		if filter == nil || filter(copy) {
			results = append(results, copy)
		}
	}
	return results
}

func (r *Registry) initMetrics(ctx context.Context) error {
	if r.meter == nil {
		return nil
	}
	gauge, err := r.meter.Int64ObservableGauge("loqa.capabilities.nodes", metric.WithDescription("Number of known nodes"))
	if err != nil {
		return err
	}
	capGauge, err := r.meter.Int64ObservableGauge("loqa.capabilities.total", metric.WithDescription("Total advertised capabilities"))
	if err != nil {
		return err
	}
	r.nodeGauge = gauge
	r.attrGauge = capGauge
	_, err = r.meter.RegisterCallback(func(ctx context.Context, obs metric.Observer) error {
		nodes, caps := r.snapshotCounts()
		obs.ObserveInt64(gauge, nodes)
		obs.ObserveInt64(capGauge, caps)
		return nil
	}, gauge, capGauge)
	return err
}

func (r *Registry) snapshotCounts() (int64, int64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var nodes int64
	var caps int64
	for _, node := range r.nodes {
		nodes++
		caps += int64(len(node.Capabilities))
	}
	return nodes, caps
}

func (r *Registry) LocalCapabilities() []Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if node, ok := r.nodes[r.cfg.ID]; ok {
		return append([]Capability(nil), node.Capabilities...)
	}
	return nil
}

func convertCapabilities(source []config.NodeCapability) []Capability {
	if len(source) == 0 {
		return nil
	}
	result := make([]Capability, 0, len(source))
	for _, cap := range source {
		result = append(result, Capability{
			Name:       cap.Name,
			Tier:       cap.Tier,
			Attributes: cap.Attributes,
		})
	}
	return result
}

func WithCapabilityFilter(name string) func(NodeInfo) bool {
	return func(node NodeInfo) bool {
		for _, cap := range node.Capabilities {
			if cap.Name == name {
				return true
			}
		}
		return false
	}
}

func WithTierFilter(tier string) func(NodeInfo) bool {
	return func(node NodeInfo) bool {
		for _, cap := range node.Capabilities {
			if cap.Tier == tier {
				return true
			}
		}
		return false
	}
}

func (c Capability) AttributesAsAttrs() []attribute.KeyValue {
	var attrs []attribute.KeyValue
	for k, v := range c.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}
	return attrs
}
