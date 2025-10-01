package eventstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/loqalabs/loqa-core/internal/config"
	_ "modernc.org/sqlite"
)

// Event represents a recorded timeline entry.
type Event struct {
	ID        int64
	SessionID string
	TraceID   string
	ActorID   string
	Type      string
	Payload   []byte
	Privacy   string
	CreatedAt time.Time
}

// Store wraps a SQLite-backed event timeline store.
type Store struct {
	db    *sql.DB
	cfg   config.EventStoreConfig
	log   *slog.Logger
	clock func() time.Time
}

// Open initializes the event store according to config.
func Open(ctx context.Context, cfg config.EventStoreConfig, log *slog.Logger) (*Store, error) {
	if cfg.RetentionMode == "ephemeral" {
		return &Store{cfg: cfg, log: log, clock: time.Now}, nil
	}

	dir := filepath.Dir(cfg.Path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", cfg.Path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: db, cfg: cfg, log: log, clock: time.Now}

	if err := s.initSchema(ctx); err != nil {
		db.Close()
		return nil, err
	}

	if cfg.VacuumOnStart {
		if err := s.vacuum(ctx); err != nil {
			log.Warn("event store vacuum failed", slog.String("error", err.Error()))
		}
	}

	if err := s.Prune(ctx); err != nil {
		log.Warn("event store prune on start failed", slog.String("error", err.Error()))
	}

	return s, nil
}

func (s *Store) initSchema(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	ddl := `
CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,
    actor_id TEXT,
    privacy_scope TEXT,
    created_at TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    trace_id TEXT,
    actor_id TEXT,
    event_type TEXT,
    payload BLOB,
    privacy_scope TEXT,
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY(session_id) REFERENCES sessions(session_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_events_session_created ON events(session_id, created_at);
`
	_, err := s.db.ExecContext(ctx, ddl)
	return err
}

func (s *Store) vacuum(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, "VACUUM")
	return err
}

// Close releases underlying resources.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// AppendSession ensures a session row exists.
func (s *Store) AppendSession(ctx context.Context, sessionID, actorID, privacy string) error {
	if s.cfg.RetentionMode == "ephemeral" || s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions(session_id, actor_id, privacy_scope, created_at)
		 VALUES(?, ?, ?, ?)
		 ON CONFLICT(session_id) DO UPDATE SET actor_id=excluded.actor_id, privacy_scope=excluded.privacy_scope`,
		sessionID, actorID, privacy, s.clock().UTC())
	return err
}

// AppendEvent writes an event into the store.
func (s *Store) AppendEvent(ctx context.Context, evt Event) error {
	if s.cfg.RetentionMode == "ephemeral" || s.db == nil {
		return nil
	}
	if evt.CreatedAt.IsZero() {
		evt.CreatedAt = s.clock().UTC()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO events(session_id, trace_id, actor_id, event_type, payload, privacy_scope, created_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)`,
		evt.SessionID, evt.TraceID, evt.ActorID, evt.Type, evt.Payload, evt.Privacy, evt.CreatedAt)
	return err
}

// ListSessionEvents retrieves up to limit events for a session ordered ascending by time.
func (s *Store) ListSessionEvents(ctx context.Context, sessionID string, limit int) ([]Event, error) {
	if s.cfg.RetentionMode == "ephemeral" || s.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, trace_id, actor_id, event_type, payload, privacy_scope, created_at
		 FROM events WHERE session_id = ? ORDER BY created_at ASC LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var created string
		if err := rows.Scan(&e.ID, &e.SessionID, &e.TraceID, &e.ActorID, &e.Type, &e.Payload, &e.Privacy, &created); err != nil {
			return nil, err
		}
		if ts, err := time.Parse(time.RFC3339Nano, created); err == nil {
			e.CreatedAt = ts
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// Prune applies configured retention (called on startup and can be scheduled).
func (s *Store) Prune(ctx context.Context) error {
	if s.cfg.RetentionMode == "ephemeral" || s.db == nil {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if s.cfg.RetentionMode != "persistent" && s.cfg.RetentionMode != "session" {
		// nothing to prune
		return tx.Commit()
	}
	if s.cfg.RetentionDays > 0 {
		cutoff := s.clock().Add(-time.Duration(s.cfg.RetentionDays) * 24 * time.Hour)
		if _, err = tx.ExecContext(ctx, `DELETE FROM events WHERE created_at < ?`, cutoff.UTC()); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE created_at < ?`, cutoff.UTC()); err != nil {
			return err
		}
	}
	if s.cfg.MaxSessions > 0 {
		_, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE session_id IN (
			SELECT session_id FROM sessions ORDER BY created_at DESC LIMIT -1 OFFSET ?
		)`, s.cfg.MaxSessions)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

// Ensure supplies a no-op store when persistence disabled.
func (s *Store) Ensure() error {
	if s.cfg.RetentionMode == "ephemeral" && s.db != nil {
		return errors.New("ephemeral store should not have database connection")
	}
	return nil
}
