package playerstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLoadOrCreateAndServerIsolation(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if _, err := store.LoadOrCreate(AccountKey{Account: "", ServerKey: "game.cn.1"}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey for empty account, got %v", err)
	}

	k1 := AccountKey{Account: "shuma_100001", ServerKey: "game.cn.1"}
	k2 := AccountKey{Account: "shuma_100001", ServerKey: "game.cn.2"}

	s1, err := store.LoadOrCreate(k1)
	if err != nil {
		t.Fatalf("LoadOrCreate k1: %v", err)
	}
	if s1.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", s1.SchemaVersion, CurrentSchemaVersion)
	}
	if s1.Role.Gold != 10000 || len(s1.Cards) != 1 {
		t.Fatalf("unexpected default save: %+v", s1)
	}

	_, err = store.Update(k1, func(s *PlayerSave) error {
		s.Role.Gold = 55555
		return nil
	})
	if err != nil {
		t.Fatalf("Update k1: %v", err)
	}

	s2, err := store.LoadOrCreate(k2)
	if err != nil {
		t.Fatalf("LoadOrCreate k2: %v", err)
	}
	if s2.Role.Gold != 10000 {
		t.Fatalf("server isolation broken: k2 gold = %d, want 10000", s2.Role.Gold)
	}

	// Re-open store from disk to verify persistence.
	store2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reopen: %v", err)
	}
	defer store2.Close()
	reloaded, err := store2.Load(k1)
	if err != nil {
		t.Fatalf("Load k1 after reopen: %v", err)
	}
	if reloaded.Role.Gold != 55555 || reloaded.SyncVersion != 2 {
		t.Fatalf("reloaded mismatch: gold=%d syncVersion=%d", reloaded.Role.Gold, reloaded.SyncVersion)
	}
}

func TestLegacySaveUpgrade(t *testing.T) {
	dir := t.TempDir()
	key := AccountKey{Account: "legacy_user", ServerKey: "game.cn.1"}

	// Write a minimal v1 save missing MapOpen, HouseLovers, AwardRecords, Skills, NextLevelExp.
	legacy := map[string]any{
		"schema_version": 1,
		"role": map[string]any{
			"id":    "100001",
			"name":  "老玩家",
			"level": 12,
			"gold":  8888,
		},
		"cards": map[string]any{
			"1001": map[string]any{
				"id":      "1001",
				"card_id": 21,
				"level":   10,
			},
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, key.Filename()), raw, 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	upgraded, err := store.Load(key)
	if err != nil {
		t.Fatalf("Load legacy: %v", err)
	}
	if upgraded.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", upgraded.SchemaVersion, CurrentSchemaVersion)
	}
	if upgraded.Role.Name != "老玩家" || upgraded.Role.Gold != 8888 || upgraded.Role.Level != 12 {
		t.Fatalf("legacy role fields lost: %+v", upgraded.Role)
	}
	if upgraded.HouseLovers == nil || upgraded.AwardRecords == nil || len(upgraded.MapOpen) == 0 || len(upgraded.GateOpen) == 0 {
		t.Fatalf("expected upgraded default collections, got %+v", upgraded)
	}
	if upgraded.Cards["1001"].NextLevelExp != 100 || upgraded.Cards["1001"].Skills == nil {
		t.Fatalf("expected upgraded card defaults, got %+v", upgraded.Cards["1001"])
	}

	// Verify upgraded schema was persisted in SQLite across reload.
	reloaded, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reload: %v", err)
	}
	defer reloaded.Close()
	diskSave, err := reloaded.Load(key)
	if err != nil {
		t.Fatalf("reload upgraded save: %v", err)
	}
	if diskSave.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("disk SchemaVersion = %d, want %d", diskSave.SchemaVersion, CurrentSchemaVersion)
	}
}

func TestUpdateCommitAndRollback(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	key := AccountKey{Account: "shuma_100001", ServerKey: "game.cn.1"}
	if _, err := store.LoadOrCreate(key); err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}

	// Atomic state + award update.
	updated, err := store.Update(key, func(s *PlayerSave) error {
		if err := s.ClaimAward("guide:103"); err != nil {
			return err
		}
		s.Role.Gold += 500
		s.Items[501] += 2
		s.HouseLovers[1] = HouseLoverRecord{Feel: 20, Card: "1001"}
		return nil
	})
	if err != nil {
		t.Fatalf("Update commit: %v", err)
	}
	if updated.Role.Gold != 10500 || updated.Items[501] != 2 || !updated.AwardRecords["guide:103"] {
		t.Fatalf("unexpected committed save: %+v", updated)
	}

	// Failed update must roll back all mutations (including Gold/Items/SyncVersion).
	sentinel := errors.New("insufficient stamina")
	_, err = store.Update(key, func(s *PlayerSave) error {
		s.Role.Gold = 999999
		s.Items[501] = 99
		if err := s.ClaimAward("gate:10101:first"); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	// Duplicate award claim also rolls back.
	_, err = store.Update(key, func(s *PlayerSave) error {
		s.Role.Gold += 999
		return s.ClaimAward("guide:103")
	})
	if !errors.Is(err, ErrAlreadyAwarded) {
		t.Fatalf("expected ErrAlreadyAwarded, got %v", err)
	}

	afterRollback, err := store.Load(key)
	if err != nil {
		t.Fatalf("Load after rollback: %v", err)
	}
	if afterRollback.Role.Gold != 10500 || afterRollback.Items[501] != 2 || afterRollback.AwardRecords["gate:10101:first"] || afterRollback.SyncVersion != 2 {
		t.Fatalf("rollback failed, state leaked: %+v", afterRollback)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	key := AccountKey{Account: "concurrent_user", ServerKey: "game.cn.1"}
	if _, err := store.LoadOrCreate(key); err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}

	const workers = 12
	const iters = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers*iters)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				if i%2 == 0 {
					_, err := store.Update(key, func(s *PlayerSave) error {
						s.Role.Gold++
						return nil
					})
					if err != nil {
						errs <- err
					}
				} else {
					if _, err := store.Load(key); err != nil {
						errs <- err
					}
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent error: %v", err)
	}

	final, err := store.Load(key)
	if err != nil {
		t.Fatalf("final Load: %v", err)
	}
	wantGold := int64(10000 + workers*(iters/2))
	if final.Role.Gold != wantGold {
		t.Fatalf("final gold = %d, want %d", final.Role.Gold, wantGold)
	}
}
