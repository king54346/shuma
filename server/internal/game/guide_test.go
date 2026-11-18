package game

import (
	"slices"
	"testing"
)

func TestGuideAward103GrantsCardOnce(t *testing.T) {
	svc := New()
	req := map[string]any{"input": map[string]any{"guideCsvID": int64(103)}}

	award := svc.Handle("/game/role/guide/newbie/award", req)
	if award["ret"] != true {
		t.Fatalf("claim failed: %v", award)
	}
	view := award["view"].(map[string]any)
	cardID := view["carddbIDs"].([]any)[0].([]any)[0].(string)
	update := award["sync"].(map[string]any)["upd"].(map[string]any)
	card := update["cards"].(map[string]any)[cardID].(map[string]any)["_db"].(map[string]any)
	if card["card_id"] != int64(92) || card["character"] != int64(5) {
		t.Fatalf("unexpected award card: %v", card)
	}

	retry := svc.Handle("/game/role/guide/newbie/award", req)
	if retry["ret"] != true || retry["view"].(map[string]any)["carddbIDs"].([]any)[0].([]any)[0] != cardID ||
		retry["sync"].(map[string]any)["version"] != award["sync"].(map[string]any)["version"] {
		t.Fatalf("retry did not return the previous award: %v", retry)
	}

	model := svc.Handle("/game/login", nil)["model"].(map[string]any)
	role := model["role"].(map[string]any)["_db"].(map[string]any)
	cards := role["cards"].([]any)
	if len(cards) != 2 || !slices.Contains(cards, any(cardID)) {
		t.Fatalf("unexpected cards after relogin: %v", cards)
	}
	if guides := role["newbie_guide"].([]any); !slices.Contains(guides, any(int64(10))) {
		t.Fatalf("guide stage 10 was not saved: %v", guides)
	}
}
