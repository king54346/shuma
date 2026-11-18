package login

import (
	"testing"

	"shuma-server/internal/config"
)

func TestAccountRegistrationAndLoginPersistence(t *testing.T) {
	dir := t.TempDir()
	store, err := NewAccountStore(dir)
	if err != nil {
		t.Fatalf("NewAccountStore: %v", err)
	}
	defer store.Close()
	cfg := config.Default()

	// 1. First login registers new account with is_new = true.
	res1 := HandleWithAccounts(cfg, store, "/login/check", map[string]any{"name": "alice"})
	if res1["ret"] != true || res1["is_new"] != true {
		t.Fatalf("expected new registration for alice, got %#v", res1)
	}
	acct1 := res1["account"].(map[string]any)
	if acct1["id"] != int64(100001) || acct1["name"] != "alice" {
		t.Fatalf("unexpected account1: %#v", acct1)
	}

	// 2. Second account gets incremented ID 100002.
	res2 := HandleWithAccounts(cfg, store, "/login/check", map[string]any{"name": "bob"})
	if res2["ret"] != true || res2["is_new"] != true {
		t.Fatalf("expected new registration for bob, got %#v", res2)
	}
	if res2["account"].(map[string]any)["id"] != int64(100002) {
		t.Fatalf("expected bob id 100002, got %#v", res2["account"])
	}

	// 3. Reload store from disk and log in alice again -> is_new = false, same ID.
	store2, err := NewAccountStore(dir)
	if err != nil {
		t.Fatalf("NewAccountStore reload: %v", err)
	}
	defer store2.Close()
	res3 := HandleWithAccounts(cfg, store2, "/login/check", map[string]any{"name": "alice"})
	if res3["ret"] != true || res3["is_new"] != false {
		t.Fatalf("expected existing login for alice, got %#v", res3)
	}
	if res3["account"].(map[string]any)["id"] != int64(100001) {
		t.Fatalf("expected persisted alice id 100001, got %#v", res3["account"])
	}

	// 4. Enter server records pending session for game login.
	enterRes := HandleWithAccounts(cfg, store2, "/login/enter_server", map[string]any{"key": "game.cn.1"})
	if enterRes["ret"] != true || enterRes["serv_key"] != "game.cn.1" {
		t.Fatalf("enter_server failed: %#v", enterRes)
	}
	acct, serv, ok := store2.ConsumePendingSession()
	if !ok || acct.Name != "alice" || serv != "game.cn.1" {
		t.Fatalf("unexpected pending session: ok=%v acct=%+v serv=%q", ok, acct, serv)
	}
}
