package playerstore

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	// CurrentSchemaVersion is the latest persisted save format version.
	CurrentSchemaVersion = 2
)

var (
	ErrInvalidKey     = errors.New("playerstore: invalid account or server key")
	ErrSaveNotFound   = errors.New("playerstore: save not found")
	ErrAlreadyAwarded = errors.New("playerstore: award already claimed")
)

// AccountKey uniquely identifies a player save by account and server key.
type AccountKey struct {
	Account   string `json:"account"`
	ServerKey string `json:"server_key"`
}

// Validate checks that Account and ServerKey are non-empty.
func (k AccountKey) Validate() error {
	if strings.TrimSpace(k.Account) == "" || strings.TrimSpace(k.ServerKey) == "" {
		return ErrInvalidKey
	}
	return nil
}

// Filename returns a deterministic, filesystem-safe filename for the key.
func (k AccountKey) Filename() string {
	sum := sha256.Sum256([]byte(k.Account + "\x00" + k.ServerKey))
	return hex.EncodeToString(sum[:16]) + ".json"
}

// RoleState holds core trainer attributes.
type RoleState struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Level                  int64  `json:"level"`
	LevelExp               int64  `json:"level_exp"`
	VIPLevel               int64  `json:"vip_level"`
	Gold                   int64  `json:"gold"`
	RMB                    int64  `json:"rmb"`
	Stamina                int64  `json:"stamina"`
	StaminaLastRecoverTime int64  `json:"stamina_last_recover_time"`
	Figure                 int64  `json:"figure"`
	CreatedTime            int64  `json:"created_time"`
}

// CardRecord holds a single owned card's persisted state.
type CardRecord struct {
	ID            string          `json:"id"`
	CardID        int64           `json:"card_id"`
	Level         int64           `json:"level"`
	Star          int64           `json:"star"`
	Advance       int64           `json:"advance"`
	Exp           int64           `json:"exp"`
	LevelExp      int64           `json:"level_exp"`
	NextLevelExp  int64           `json:"next_level_exp"`
	FightingPoint int64           `json:"fighting_point"`
	Skills        map[int64]int64 `json:"skills"`
	Locked        bool            `json:"locked"`
	Character     int64           `json:"character"`
	SkinID        int64           `json:"skin_id"`
	Extra         map[string]any  `json:"extra,omitempty"`
}

type MenuRecord struct {
	Species        map[int64]int64       `json:"species,omitempty"`
	PokedexAdvance map[int64]int64       `json:"pokedex_advance,omitempty"`
	Draw           DrawRecord            `json:"draw"`
	Shops          map[string]ShopRecord `json:"shops,omitempty"`
	Ready          map[int64]TeamRecord  `json:"ready,omitempty"`
	Defence        []string              `json:"defence,omitempty"`
	TaskDay        int64                 `json:"task_day"`
	GateAttempts   int64                 `json:"gate_attempts"`
	DailyClaims    map[int64]int64       `json:"daily_claims,omitempty"`
	MainClaims     map[int64]bool        `json:"main_claims,omitempty"`
	LivenessClaims map[int64]int64       `json:"liveness_claims,omitempty"`
	Liveness       int64                 `json:"liveness"`
	FragmentCoins  int64                 `json:"fragment_coins"`
}

type DrawRecord struct {
	Day, FreeDiamond, FreeGold, LastFreeGold, HalfDiamond, DailyTotal int64
}

type ShopRecord struct {
	Cycle, Updated int64
	Bought         map[int64]int64
}

type TeamRecord struct {
	Name  string
	Cards []string
}

// GateStarRecord stores stars and chest claim state for a cleared gate.
type GateStarRecord struct {
	Star  int64 `json:"star"`
	Chest int64 `json:"chest"`
}

// HouseLoverRecord stores affection and blessed card for a house lover.
type HouseLoverRecord struct {
	Feel int64  `json:"feel"`
	Card string `json:"card,omitempty"`
}

type SignRecord struct {
	Month       int64           `json:"month"`
	Count       int64           `json:"count"`
	LastDay     int64           `json:"last_day"`
	LastIndex   int64           `json:"last_index"`
	Buys        int64           `json:"buys"`
	Total       int64           `json:"total"`
	Claimed     int64           `json:"claimed"`
	MonthGifts  map[int64]int64 `json:"month_gifts"`
	DailyAwards map[int64]int64 `json:"daily_awards"`
}

// PlayerSave is the versioned, persisted state for one (account, server_key).
type PlayerSave struct {
	SchemaVersion int                        `json:"schema_version"`
	Key           AccountKey                 `json:"key"`
	SyncVersion   int64                      `json:"sync_version"`
	UpdatedAt     int64                      `json:"updated_at"`
	Role          RoleState                  `json:"role"`
	Items         map[int64]int64            `json:"items"`
	NewbieGuide   []int64                    `json:"newbie_guide"`
	AwardRecords  map[string]bool            `json:"award_records"`
	Cards         map[string]CardRecord      `json:"cards"`
	BattleCards   []string                   `json:"battle_cards"`
	GateStar      map[int64]GateStarRecord   `json:"gate_star"`
	GateOpen      []int64                    `json:"gate_open"`
	MapOpen       []int64                    `json:"map_open"`
	HouseLovers   map[int64]HouseLoverRecord `json:"house_lovers"`
	Fragments     map[int64]int64            `json:"fragments,omitempty"`
	GuideAwards   map[int64]string           `json:"guide_awards,omitempty"`
	ChapterClaims map[int64]bool             `json:"chapter_claims,omitempty"`
	Sign          SignRecord                 `json:"sign"`
	Menu          MenuRecord                 `json:"menu"`
}

// Clone returns a deep copy of PlayerSave so callers and failed transactions cannot mutate stored state.
func (s PlayerSave) Clone() PlayerSave {
	out := s
	out.Items = maps.Clone(s.Items)
	out.NewbieGuide = slices.Clone(s.NewbieGuide)
	out.AwardRecords = maps.Clone(s.AwardRecords)
	out.BattleCards = slices.Clone(s.BattleCards)
	out.GateStar = maps.Clone(s.GateStar)
	out.GateOpen = slices.Clone(s.GateOpen)
	out.MapOpen = slices.Clone(s.MapOpen)
	out.HouseLovers = maps.Clone(s.HouseLovers)
	out.Fragments = maps.Clone(s.Fragments)
	out.GuideAwards = maps.Clone(s.GuideAwards)
	out.ChapterClaims = maps.Clone(s.ChapterClaims)
	out.Sign.MonthGifts = maps.Clone(s.Sign.MonthGifts)
	out.Sign.DailyAwards = maps.Clone(s.Sign.DailyAwards)
	out.Menu.Species = maps.Clone(s.Menu.Species)
	out.Menu.PokedexAdvance = maps.Clone(s.Menu.PokedexAdvance)
	out.Menu.DailyClaims = maps.Clone(s.Menu.DailyClaims)
	out.Menu.MainClaims = maps.Clone(s.Menu.MainClaims)
	out.Menu.LivenessClaims = maps.Clone(s.Menu.LivenessClaims)
	out.Menu.Defence = slices.Clone(s.Menu.Defence)
	out.Menu.Shops = maps.Clone(s.Menu.Shops)
	for key, shop := range out.Menu.Shops {
		shop.Bought = maps.Clone(shop.Bought)
		out.Menu.Shops[key] = shop
	}
	out.Menu.Ready = maps.Clone(s.Menu.Ready)
	for key, team := range out.Menu.Ready {
		team.Cards = slices.Clone(team.Cards)
		out.Menu.Ready[key] = team
	}

	if s.Cards != nil {
		out.Cards = make(map[string]CardRecord, len(s.Cards))
		for k, c := range s.Cards {
			cCopy := c
			cCopy.Skills = maps.Clone(c.Skills)
			if c.Extra != nil {
				cCopy.Extra = cloneCardExtras(c.Extra).(map[string]any)
			}
			out.Cards[k] = cCopy
		}
	}
	return out
}

func cloneCardExtras(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			out[key] = cloneCardExtras(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = cloneCardExtras(child)
		}
		return out
	default:
		return value
	}
}

// ClaimAward marks awardKey as claimed, returning ErrAlreadyAwarded if it was already recorded.
func (s *PlayerSave) ClaimAward(awardKey string) error {
	if s.AwardRecords == nil {
		s.AwardRecords = make(map[string]bool)
	}
	if s.AwardRecords[awardKey] {
		return fmt.Errorf("%w: %s", ErrAlreadyAwarded, awardKey)
	}
	s.AwardRecords[awardKey] = true
	return nil
}

// NewDefaultSave creates a fresh v2 PlayerSave for the given AccountKey.
func NewDefaultSave(key AccountKey, now int64) PlayerSave {
	if now <= 0 {
		now = time.Now().Unix()
	}
	return PlayerSave{
		SchemaVersion: CurrentSchemaVersion,
		Key:           key,
		SyncVersion:   1,
		UpdatedAt:     now,
		Role: RoleState{
			ID:                     "100001",
			Name:                   "本地测试训练师",
			Level:                  1,
			LevelExp:               0,
			VIPLevel:               0,
			Gold:                   10000,
			RMB:                    1000,
			Stamina:                120,
			StaminaLastRecoverTime: now,
			Figure:                 1,
			CreatedTime:            now,
		},
		Items:        map[int64]int64{},
		NewbieGuide:  []int64{-1, 1, 2},
		AwardRecords: map[string]bool{},
		Cards: map[string]CardRecord{
			"1001": {
				ID:            "1001",
				CardID:        21,
				Level:         1,
				Star:          1,
				Advance:       1,
				Exp:           0,
				LevelExp:      0,
				NextLevelExp:  100,
				FightingPoint: 500,
				Skills:        map[int64]int64{},
				Character:     1,
			},
		},
		BattleCards: []string{"1001"},
		GateStar:    map[int64]GateStarRecord{},
		GateOpen:    []int64{10101},
		MapOpen:     []int64{11},
		HouseLovers: map[int64]HouseLoverRecord{},
	}
}

// migrateSave upgrades v0/v1 saves in place to CurrentSchemaVersion.
func migrateSave(save *PlayerSave, key AccountKey, now int64) bool {
	upgraded := false
	if save.SchemaVersion < CurrentSchemaVersion {
		upgraded = true
		save.SchemaVersion = CurrentSchemaVersion
	}
	if save.Key.Account == "" {
		save.Key = key
		upgraded = true
	}
	if save.SyncVersion <= 0 {
		save.SyncVersion = 1
		upgraded = true
	}
	if save.Role.ID == "" {
		save.Role.ID = "100001"
		upgraded = true
	}
	if save.Role.Name == "" {
		save.Role.Name = "本地测试训练师"
		upgraded = true
	}
	if save.Role.Level <= 0 {
		save.Role.Level = 1
		upgraded = true
	}
	if save.Role.CreatedTime <= 0 {
		save.Role.CreatedTime = now
		upgraded = true
	}
	if save.Role.StaminaLastRecoverTime <= 0 {
		save.Role.StaminaLastRecoverTime = now
		upgraded = true
	}
	if save.Items == nil {
		save.Items = map[int64]int64{}
		upgraded = true
	}
	if save.NewbieGuide == nil {
		save.NewbieGuide = []int64{}
		upgraded = true
	}
	if save.AwardRecords == nil {
		save.AwardRecords = map[string]bool{}
		upgraded = true
	}
	if save.Cards == nil {
		save.Cards = map[string]CardRecord{}
		upgraded = true
	}
	for id, c := range save.Cards {
		if c.Skills == nil {
			c.Skills = map[int64]int64{}
			save.Cards[id] = c
			upgraded = true
		}
		if c.NextLevelExp <= 0 {
			c.NextLevelExp = 100
			save.Cards[id] = c
			upgraded = true
		}
	}
	if save.BattleCards == nil {
		save.BattleCards = []string{}
		upgraded = true
	}
	if save.GateStar == nil {
		save.GateStar = map[int64]GateStarRecord{}
		upgraded = true
	}
	if len(save.GateOpen) == 0 {
		save.GateOpen = []int64{10101}
		upgraded = true
	}
	if len(save.MapOpen) == 0 {
		save.MapOpen = []int64{11}
		upgraded = true
	}
	if save.HouseLovers == nil {
		save.HouseLovers = map[int64]HouseLoverRecord{}
		upgraded = true
	}
	return upgraded
}

type lockEntry struct {
	mu sync.Mutex
}

// Store provides thread-safe, SQLite-backed persistence for PlayerSave records.
type Store struct {
	dir     string
	db      *sql.DB
	nowFunc func() int64

	mapMu sync.Mutex
	locks map[AccountKey]*lockEntry
	cache map[AccountKey]PlayerSave
}

// NewStore creates a Store backed by <dir>/shuma.db (SQLite), creating dir if needed.
func NewStore(dir string) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("playerstore: empty directory")
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
		CREATE TABLE IF NOT EXISTS player_saves (
			account TEXT NOT NULL,
			server_key TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			sync_version INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			payload TEXT NOT NULL,
			PRIMARY KEY (account, server_key)
		);
	`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{
		dir:     dir,
		db:      db,
		nowFunc: func() int64 { return time.Now().Unix() },
		locks:   make(map[AccountKey]*lockEntry),
		cache:   make(map[AccountKey]PlayerSave),
	}, nil
}

// Close closes the underlying SQLite database.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) lockFor(key AccountKey) *lockEntry {
	s.mapMu.Lock()
	defer s.mapMu.Unlock()
	entry, ok := s.locks[key]
	if !ok {
		entry = &lockEntry{}
		s.locks[key] = entry
	}
	return entry
}

func (s *Store) pathFor(key AccountKey) string {
	return filepath.Join(s.dir, key.Filename())
}

// LoadOrCreate loads an existing save (upgrading old schemas automatically) or initializes and persists a new save.
func (s *Store) LoadOrCreate(key AccountKey) (PlayerSave, error) {
	if err := key.Validate(); err != nil {
		return PlayerSave{}, err
	}
	entry := s.lockFor(key)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	save, err := s.loadUnlocked(key)
	if err == nil {
		return save.Clone(), nil
	}
	if !errors.Is(err, ErrSaveNotFound) {
		return PlayerSave{}, err
	}

	save = NewDefaultSave(key, s.nowFunc())
	if err := s.writeAtomic(key, save); err != nil {
		return PlayerSave{}, err
	}
	s.setCache(key, save)
	return save.Clone(), nil
}

// Load reads an existing save without creating a new one.
func (s *Store) Load(key AccountKey) (PlayerSave, error) {
	if err := key.Validate(); err != nil {
		return PlayerSave{}, err
	}
	entry := s.lockFor(key)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	save, err := s.loadUnlocked(key)
	if err != nil {
		return PlayerSave{}, err
	}
	return save.Clone(), nil
}

// Update atomically applies fn to the save for key, persisting state and award changes together.
// If fn returns an error or persistence fails, the previous save state is preserved unchanged.
func (s *Store) Update(key AccountKey, fn func(*PlayerSave) error) (PlayerSave, error) {
	if err := key.Validate(); err != nil {
		return PlayerSave{}, err
	}
	entry := s.lockFor(key)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	cur, err := s.loadUnlocked(key)
	if err != nil {
		if !errors.Is(err, ErrSaveNotFound) {
			return PlayerSave{}, err
		}
		cur = NewDefaultSave(key, s.nowFunc())
	}

	candidate := cur.Clone()
	if err := fn(&candidate); err != nil {
		return cur.Clone(), err
	}
	candidate.SchemaVersion = CurrentSchemaVersion
	candidate.Key = key
	candidate.SyncVersion++
	candidate.UpdatedAt = s.nowFunc()

	if err := s.writeAtomic(key, candidate); err != nil {
		return cur.Clone(), err
	}
	s.setCache(key, candidate)
	return candidate.Clone(), nil
}

func (s *Store) setCache(key AccountKey, save PlayerSave) {
	s.mapMu.Lock()
	s.cache[key] = save.Clone()
	s.mapMu.Unlock()
}

func (s *Store) loadUnlocked(key AccountKey) (PlayerSave, error) {
	s.mapMu.Lock()
	cached, ok := s.cache[key]
	s.mapMu.Unlock()
	if ok {
		return cached.Clone(), nil
	}

	var raw string
	err := s.db.QueryRow(
		`SELECT payload FROM player_saves WHERE account = ? AND server_key = ?`,
		key.Account, key.ServerKey,
	).Scan(&raw)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return PlayerSave{}, err
		}
		// Migrate legacy JSON save file into SQLite if present.
		legacyRaw, rerr := os.ReadFile(s.pathFor(key))
		if rerr != nil {
			if errors.Is(rerr, os.ErrNotExist) {
				return PlayerSave{}, ErrSaveNotFound
			}
			return PlayerSave{}, rerr
		}
		raw = string(legacyRaw)
	}

	var save PlayerSave
	if err := json.Unmarshal([]byte(raw), &save); err != nil {
		return PlayerSave{}, fmt.Errorf("playerstore: decode %s: %w", key.Filename(), err)
	}
	if migrateSave(&save, key, s.nowFunc()) || errors.Is(err, sql.ErrNoRows) {
		if err := s.writeAtomic(key, save); err != nil {
			return PlayerSave{}, err
		}
	}
	s.setCache(key, save)
	return save, nil
}

func (s *Store) writeAtomic(key AccountKey, save PlayerSave) error {
	data, err := json.Marshal(save)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if _, err := tx.Exec(`
		INSERT INTO player_saves (account, server_key, schema_version, sync_version, updated_at, payload)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(account, server_key) DO UPDATE SET
			schema_version = excluded.schema_version,
			sync_version = excluded.sync_version,
			updated_at = excluded.updated_at,
			payload = excluded.payload
	`, key.Account, key.ServerKey, save.SchemaVersion, save.SyncVersion, save.UpdatedAt, string(data)); err != nil {
		return err
	}
	return tx.Commit()
}
