package game

import "testing"

func TestLocalRankAndEmptyFriends(t *testing.T) {
	p := New()
	settle(t, p, openBattle(t, p, 10101), 10101, "win", 3)
	for _, tt := range []struct {
		kind, field string
		want        int64
	}{
		{"fight", "fighting_point", 1320}, {"pokedex", "pokedex", 1}, {"star", "star", 3},
	} {
		resp := p.Handle("/game/rank", map[string]any{"type": tt.kind})
		rows := resp["view"].(map[string]any)["rank"].([]any)
		if len(rows) != 1 || rows[0].(map[string]any)[tt.field] != tt.want {
			t.Fatalf("%s: %v", tt.kind, rows)
		}
	}
	resp := p.Handle("/game/rank", map[string]any{"type": "fight", "offest": 1})
	if rows := resp["view"].(map[string]any)["rank"].([]any); rows == nil || len(rows) != 0 {
		t.Fatalf("page past end: %v", rows)
	}
	resp = p.Handle("/game/society/friend/list", nil)
	if rows := resp["view"].(map[string]any)["roles"].([]any); rows == nil || len(rows) != 0 {
		t.Fatalf("initial friends: %v", rows)
	}
}
