package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ambiware-labs/loqa-core/internal/bus"
	"github.com/ambiware-labs/loqa-core/internal/config"
	"github.com/ambiware-labs/loqa-core/internal/eventstore"
	manifestpkg "github.com/ambiware-labs/loqa-core/internal/skills/manifest"
	skillrt "github.com/ambiware-labs/loqa-core/internal/skills/runtime"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// Service manages lifecycle and execution of WASM skills.
type Service struct {
	cfg    config.SkillsConfig
	log    *slog.Logger
	bus    *bus.Client
	store  *eventstore.Store
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	sema   chan struct{}

	mu     sync.RWMutex
	skills map[string]*binding
	subs   []*nats.Subscription

	healthy bool
}

type binding struct {
	manifest      manifestpkg.Manifest
	manifestPath  string
	modulePath    string
	directory     string
	publishSet    map[string]struct{}
	subscribeList []string
	permissions   map[string]struct{}
	sessionID     string
}

// New creates the skills service. When cfg.Enabled is false, nil is returned.
func New(ctx context.Context, cfg config.SkillsConfig, busClient *bus.Client, store *eventstore.Store, logger *slog.Logger) (*Service, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if busClient == nil {
		return nil, errors.New("skills service requires bus client")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	cctx, cancel := context.WithCancel(ctx)
	svc := &Service{
		cfg:    cfg,
		log:    logger.With(slog.String("component", "skills.service")),
		bus:    busClient,
		store:  store,
		ctx:    cctx,
		cancel: cancel,
		sema:   make(chan struct{}, cfg.Concurrency),
		skills: make(map[string]*binding),
	}
	if err := svc.loadSkills(); err != nil {
		cancel()
		return nil, err
	}
	if err := svc.registerSubscriptions(); err != nil {
		svc.Close()
		return nil, err
	}
	svc.healthy = true
	return svc, nil
}

// Close terminates subscriptions and waits for in-flight executions.
func (s *Service) Close() {
	s.cancel()
	s.mu.Lock()
	for _, sub := range s.subs {
		if sub != nil {
			_ = sub.Drain()
		}
	}
	s.subs = nil
	s.mu.Unlock()
	s.wg.Wait()
}

// Healthy reports whether the service is running with active subscriptions.
func (s *Service) Healthy() bool {
	return s != nil && s.healthy
}

func (s *Service) loadSkills() error {
	root := s.cfg.Directory
	if root == "" {
		return errors.New("skills directory not configured")
	}
	entries := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "skill.yaml") {
			entries++
			if err := s.addSkill(path); err != nil {
				s.log.Error("failed to load skill", slog.String("path", path), slog.String("error", err.Error()))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(s.skills) == 0 {
		s.log.Warn("no skills discovered", slog.String("directory", root))
	} else {
		s.log.Info("skills discovered", slog.Int("count", len(s.skills)))
	}
	return nil
}

func (s *Service) addSkill(manifestPath string) error {
	mf, err := manifestpkg.Load(manifestPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}
	if err := manifestpkg.Validate(mf); err != nil {
		return fmt.Errorf("validate manifest: %w", err)
	}
	name := mf.Metadata.Name
	if name == "" {
		return errors.New("manifest missing metadata.name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.skills[name]; exists {
		return fmt.Errorf("duplicate skill name %s", name)
	}

	baseDir := filepath.Dir(manifestPath)
	modulePath := mf.Runtime.Module
	if !filepath.IsAbs(modulePath) {
		modulePath = filepath.Join(baseDir, modulePath)
	}

	publishSet := make(map[string]struct{}, len(mf.Capabilities.Bus.Publish))
	for _, subj := range mf.Capabilities.Bus.Publish {
		publishSet[subj] = struct{}{}
	}
	permSet := make(map[string]struct{}, len(mf.Permissions))
	for _, perm := range mf.Permissions {
		permSet[perm] = struct{}{}
	}

	binding := &binding{
		manifest:      mf,
		manifestPath:  manifestPath,
		modulePath:    modulePath,
		directory:     baseDir,
		publishSet:    publishSet,
		subscribeList: append([]string(nil), mf.Capabilities.Bus.Subscribe...),
		permissions:   permSet,
		sessionID:     fmt.Sprintf("skill:%s", name),
	}

	s.skills[name] = binding
	return nil
}

func (s *Service) registerSubscriptions() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, binding := range s.skills {
		for _, subject := range binding.subscribeList {
			subject := subject
			handler := s.makeHandler(binding)
			sub, err := s.bus.Conn().Subscribe(subject, handler)
			if err != nil {
				return fmt.Errorf("subscribe %s: %w", subject, err)
			}
			s.subs = append(s.subs, sub)
			s.log.Info("skill subscribed", slog.String("skill", binding.manifest.Metadata.Name), slog.String("subject", subject))
		}
	}
	return nil
}

func (s *Service) makeHandler(binding *binding) nats.MsgHandler {
	return func(msg *nats.Msg) {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.sema <- struct{}{}
			defer func() { <-s.sema }()
			if err := s.invoke(binding, msg); err != nil {
				s.log.Error("skill invocation failed", slog.String("skill", binding.manifest.Metadata.Name), slog.String("subject", msg.Subject), slog.String("error", err.Error()))
			}
		}()
	}
}

func (s *Service) invoke(binding *binding, msg *nats.Msg) error {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	invocationID := uuid.NewString()
	env := map[string]string{
		"LOQA_SKILL_NAME":      binding.manifest.Metadata.Name,
		"LOQA_EVENT_SUBJECT":   msg.Subject,
		"LOQA_EVENT_PAYLOAD":   string(msg.Data),
		"LOQA_INVOCATION_ID":   invocationID,
		"LOQA_SKILL_DIRECTORY": binding.directory,
	}
	if msg.Reply != "" {
		env["LOQA_EVENT_REPLY"] = msg.Reply
	}

	hostLogger := s.log.With(
		slog.String("skill", binding.manifest.Metadata.Name),
		slog.String("invocation_id", invocationID),
	)

	hostBindings := skillrt.HostBindings{
		Logger: hostLogger,
		AllowPublish: func(subject string) error {
			if _, ok := binding.permissions["bus:publish"]; !ok {
				return fmt.Errorf("missing permission bus:publish")
			}
			if _, ok := binding.publishSet[subject]; !ok {
				return fmt.Errorf("subject %s not declared in manifest", subject)
			}
			return nil
		},
		Publish: func(subject string, payload []byte) error {
			return s.bus.Conn().Publish(subject, payload)
		},
		RecordAudit: func(event skillrt.AuditEvent) {
			s.appendAudit(binding, invocationID, event)
		},
	}

	runtime, err := skillrt.New(ctx, hostBindings)
	if err != nil {
		return fmt.Errorf("init runtime: %w", err)
	}
	defer runtime.Close(ctx)

	mf := binding.manifest
	mf.Runtime.Module = binding.modulePath

	skill, err := runtime.Load(ctx, mf, env)
	if err != nil {
		return fmt.Errorf("load skill: %w", err)
	}
	defer skill.Close(ctx)

	start := time.Now()
	s.appendAudit(binding, invocationID, skillrt.AuditEvent{Type: "skill.invoke.start", Data: map[string]any{
		"subject": msg.Subject,
	}})

	if err := skill.Invoke(ctx); err != nil {
		s.appendAudit(binding, invocationID, skillrt.AuditEvent{Type: "skill.invoke.error", Data: map[string]any{
			"error": err.Error(),
		}})
		return err
	}

	s.appendAudit(binding, invocationID, skillrt.AuditEvent{Type: "skill.invoke.complete", Data: map[string]any{
		"duration_ms": time.Since(start).Milliseconds(),
	}})
	return nil
}

func (s *Service) appendAudit(binding *binding, invocationID string, event skillrt.AuditEvent) {
	if s.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = s.store.AppendSession(ctx, binding.sessionID, binding.manifest.Metadata.Name, s.cfg.AuditPrivacy)
	payload := map[string]any{
		"invocation_id": invocationID,
		"skill":         binding.manifest.Metadata.Name,
	}
	for k, v := range event.Data {
		payload[k] = v
	}
	data, err := json.Marshal(payload)
	if err != nil {
		s.log.Warn("failed to marshal audit event", slog.String("error", err.Error()))
		return
	}
	evt := eventstore.Event{
		SessionID: binding.sessionID,
		ActorID:   binding.manifest.Metadata.Name,
		Type:      event.Type,
		Payload:   data,
		Privacy:   s.cfg.AuditPrivacy,
	}
	if err := s.store.AppendEvent(ctx, evt); err != nil {
		s.log.Warn("failed to append audit event", slog.String("error", err.Error()))
	}
}
