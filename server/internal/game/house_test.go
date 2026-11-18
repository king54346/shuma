package game

import "testing"

func TestHouseLoversUnlockGiftAndBless(t *testing.T) {
	svc := New()
	loginRole := func() map[string]any {
		model := svc.Handle("/game/login", nil)["model"].(map[string]any)
		return model["role"].(map[string]any)["_db"].(map[string]any)
	}
	if empty, ok := loginRole()["house_lovers"].(map[int64]any); !ok || empty == nil || len(empty) != 0 {
		t.Fatalf("new role must expose an empty house table: %v", empty)
	}
	gift := map[string]any{"loveID": 1, "itemID": 9004, "itemCount": 3}
	if resp := svc.Handle("/game/lover/add/feels", gift); resp["err"] != "lover_locked" {
		t.Fatalf("locked lover: %v", resp)
	}
	for _, gate := range []int64{10101, 10102} {
		settle(t, svc, openBattle(t, svc, gate), gate, "win", 3)
	}
	if resp := svc.Handle("/game/lover/add/feels", gift); resp["err"] != "insufficient_items" || len(svc.houseLovers) != 0 {
		t.Fatalf("gift without inventory mutated house: %v", resp)
	}
	if resp := svc.Handle("/game/role/guide/newbie/award", map[string]any{"guideCsvID": 164}); resp["ret"] != true {
		t.Fatalf("guide gift: %v", resp)
	}
	if resp := svc.Handle("/game/lover/add/feels", gift); resp["ret"] != true || svc.items[9004] != int64(0) {
		t.Fatalf("gift did not update house and inventory: %v", resp)
	}
	bless := map[string]any{"loveID": 1, "cardID": "1001"}
	if resp := svc.Handle("/game/lover/bless/card", bless); resp["ret"] != true {
		t.Fatalf("bless: %v", resp)
	}
	lover := loginRole()["house_lovers"].(map[int64]any)[1].(map[string]any)
	if lover["card"] != "1001" || lover["feel"] != int64(300) {
		t.Fatalf("lost lover state on login: %v", lover)
	}
}
