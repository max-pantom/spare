package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/spare-run/spare/internal/model"
	_ "modernc.org/sqlite"
)

type Store struct {
	db          *sql.DB
	subscribers map[chan model.Event]struct{}
	subMu       sync.Mutex
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db, subscribers: map[chan model.Event]struct{}{}}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS metadata (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS machine (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	data BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS instances (
	id TEXT PRIMARY KEY,
	data BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	instance_id TEXT NOT NULL DEFAULT '',
	level TEXT NOT NULL,
	kind TEXT NOT NULL,
	message TEXT NOT NULL,
	details BLOB,
	created_at TEXT NOT NULL
);
INSERT INTO metadata(key, value) VALUES ('schema_version', '2')
	ON CONFLICT(key) DO UPDATE SET value = excluded.value;
`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *Store) SaveMachine(ctx context.Context, machine model.Machine) error {
	data, err := json.Marshal(machine)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO machine(id, data) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data
	`, data)
	return err
}

func (s *Store) Machine(ctx context.Context) (model.Machine, error) {
	var data []byte
	if err := s.db.QueryRowContext(ctx, `SELECT data FROM machine WHERE id = 1`).Scan(&data); err != nil {
		return model.Machine{}, err
	}
	var machine model.Machine
	if err := json.Unmarshal(data, &machine); err != nil {
		return model.Machine{}, err
	}
	return machine, nil
}

func (s *Store) PutInstance(ctx context.Context, instance model.Instance) error {
	data, err := json.Marshal(instance)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO instances(id, data) VALUES (?, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data
	`, instance.ID, data)
	return err
}

func (s *Store) Instance(ctx context.Context, id string) (model.Instance, error) {
	var data []byte
	if err := s.db.QueryRowContext(ctx, `SELECT data FROM instances WHERE id = ?`, id).Scan(&data); err != nil {
		return model.Instance{}, err
	}
	var instance model.Instance
	if err := json.Unmarshal(data, &instance); err != nil {
		return model.Instance{}, err
	}
	return instance, nil
}

func (s *Store) Instances(ctx context.Context) ([]model.Instance, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT data FROM instances ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.Instance
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var instance model.Instance
		if err := json.Unmarshal(data, &instance); err != nil {
			return nil, err
		}
		result = append(result, instance)
	}
	return result, rows.Err()
}

func (s *Store) DeleteInstance(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM instances WHERE id = ?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) AddEvent(ctx context.Context, event model.Event) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	details, err := json.Marshal(event.Details)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO events(instance_id, level, kind, message, details, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, event.InstanceID, event.Level, event.Kind, event.Message, details, event.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	event.ID, _ = result.LastInsertId()
	s.publishEvent(event)
	return err
}

// SubscribeEvents reports newly committed events until ctx is cancelled. A
// slow observer may miss an event and should refresh the events endpoint.
func (s *Store) SubscribeEvents(ctx context.Context) <-chan model.Event {
	events := make(chan model.Event, 16)
	s.subMu.Lock()
	s.subscribers[events] = struct{}{}
	s.subMu.Unlock()
	go func() {
		<-ctx.Done()
		s.subMu.Lock()
		delete(s.subscribers, events)
		close(events)
		s.subMu.Unlock()
	}()
	return events
}

func (s *Store) publishEvent(event model.Event) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for subscriber := range s.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (s *Store) Events(ctx context.Context, limit int) ([]model.Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, instance_id, level, kind, message, details, created_at
		FROM events ORDER BY id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.Event
	for rows.Next() {
		var event model.Event
		var details []byte
		var created string
		if err := rows.Scan(
			&event.ID,
			&event.InstanceID,
			&event.Level,
			&event.Kind,
			&event.Message,
			&details,
			&created,
		); err != nil {
			return nil, err
		}
		if len(details) > 0 && string(details) != "null" {
			if err := json.Unmarshal(details, &event.Details); err != nil {
				return nil, fmt.Errorf("decode event details: %w", err)
			}
		}
		event.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		result = append(result, event)
	}
	return result, rows.Err()
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
