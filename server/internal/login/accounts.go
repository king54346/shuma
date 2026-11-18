package login

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type RoleSummary struct {
	Name   string `json:"name"`
	Level  int64  `json:"level"`
	Figure int64  `json:"figure"`
}

type AccountRecord struct {
	ID          int64                  `json:"id"`
	Name        string                 `json:"name"`
	Channel     string                 `json:"channel"`
	CreatedAt   int64                  `json:"created_at"`
	LastLoginAt int64                  `json:"last_login_at"`
	RoleInfos   map[string]RoleSummary `json:"role_infos,omitempty"`
}

type legacyAccountFile struct {
	NextID   int64                    `json:"next_id"`
	Accounts map[string]AccountRecord `json:"accounts"`
}

// AccountStore manages persistent account registration, login state, and pending game handoffs in SQLite.
type AccountStore struct {
	mu          sync.Mutex
	db          *sql.DB
	lastChecked AccountRecord
	hasChecked  bool
	pendingAcct AccountRecord
	pendingServ string
	hasPending  bool
}

// NewAccountStore opens <dir>/shuma.db and initializes the accounts table.
func NewAccountStore(dir string) (*AccountStore, error) {
	if dir == "" {
		dir = "saves"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dir, "shuma.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA busy_timeout=5000;
		CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			channel TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			last_login_at INTEGER NOT NULL,
			role_infos TEXT NOT NULL
		);
	`); err != nil {
		_ = db.Close()
		return nil, err
	}

	s := &AccountStore{db: db}
	if err := s.migrateLegacyJSON(filepath.Join(dir, "accounts.json")); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying SQLite database.
func (s *AccountStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *AccountStore) migrateLegacyJSON(legacyPath string) error {
	raw, err := os.ReadFile(legacyPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var af legacyAccountFile
	if err := json.Unmarshal(raw, &af); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	for _, rec := range af.Accounts {
		riBytes, _ := json.Marshal(rec.RoleInfos)
		if len(riBytes) == 0 {
			riBytes = []byte("{}")
		}
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO accounts (id, name, channel, created_at, last_login_at, role_infos)
			VALUES (?, ?, ?, ?, ?, ?)
		`, rec.ID, rec.Name, rec.Channel, rec.CreatedAt, rec.LastLoginAt, string(riBytes)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CheckOrRegister looks up an existing account or auto-registers a new one in SQLite.
// Returns (record, isNew, err).
func (s *AccountStore) CheckOrRegister(name, channel string) (AccountRecord, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return AccountRecord{}, false, errors.New("name_can_not_empty")
	}

	now := time.Now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return AccountRecord{}, false, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var (
		rec       AccountRecord
		roleJSON  string
		isNew     bool
	)
	err = tx.QueryRow(`
		SELECT id, name, channel, created_at, last_login_at, role_infos
		FROM accounts WHERE name = ?
	`, name).Scan(&rec.ID, &rec.Name, &rec.Channel, &rec.CreatedAt, &rec.LastLoginAt, &roleJSON)

	if errors.Is(err, sql.ErrNoRows) {
		isNew = true
		var maxID sql.NullInt64
		if err := tx.QueryRow(`SELECT MAX(id) FROM accounts`).Scan(&maxID); err != nil {
			return AccountRecord{}, false, err
		}
		nextID := int64(100001)
		if maxID.Valid && maxID.Int64 >= 100001 {
			nextID = maxID.Int64 + 1
		}
		rec = AccountRecord{
			ID:          nextID,
			Name:        name,
			Channel:     channel,
			CreatedAt:   now,
			LastLoginAt: now,
			RoleInfos:   map[string]RoleSummary{},
		}
		if _, err := tx.Exec(`
			INSERT INTO accounts (id, name, channel, created_at, last_login_at, role_infos)
			VALUES (?, ?, ?, ?, ?, ?)
		`, rec.ID, rec.Name, rec.Channel, rec.CreatedAt, rec.LastLoginAt, "{}"); err != nil {
			return AccountRecord{}, false, err
		}
	} else if err != nil {
		return AccountRecord{}, false, err
	} else {
		if channel != "" {
			rec.Channel = channel
		}
		rec.LastLoginAt = now
		if roleJSON != "" {
			_ = json.Unmarshal([]byte(roleJSON), &rec.RoleInfos)
		}
		if rec.RoleInfos == nil {
			rec.RoleInfos = map[string]RoleSummary{}
		}
		if _, err := tx.Exec(`
			UPDATE accounts SET channel = ?, last_login_at = ? WHERE id = ?
		`, rec.Channel, rec.LastLoginAt, rec.ID); err != nil {
			return AccountRecord{}, false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return AccountRecord{}, false, err
	}

	s.lastChecked = rec
	s.hasChecked = true
	return rec, isNew, nil
}

// BeginEnterServer records the selected server for the most recently checked account.
func (s *AccountStore) BeginEnterServer(serverKey string) AccountRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	if serverKey == "" {
		serverKey = "game.cn.1"
	}
	if !s.hasChecked {
		s.lastChecked = AccountRecord{
			ID:        100001,
			Name:      "shuma_100001",
			RoleInfos: map[string]RoleSummary{},
		}
		s.hasChecked = true
	}
	s.pendingAcct = s.lastChecked
	s.pendingServ = serverKey
	s.hasPending = true
	return s.pendingAcct
}

// ConsumePendingSession returns the pending (account, serverKey) for /game/login.
func (s *AccountStore) ConsumePendingSession() (AccountRecord, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.hasPending {
		if s.hasChecked {
			return s.lastChecked, "game.cn.1", true
		}
		return AccountRecord{}, "", false
	}
	acct := s.pendingAcct
	serv := s.pendingServ
	s.hasPending = false
	return acct, serv, true
}
