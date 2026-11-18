package game

import "testing"

func TestDrawCardSendsPotentialAttributes(t *testing.T) {
	svc := New()
	resp := svc.Handle("/game/lottery/card/draw", map[string]any{
		"input": map[string]any{"drawType": "free1"},
	})
	view := resp["view"].(map[string]any)["result"].(map[string]any)
	id := view["carddbIDs"].([]any)[0].([]any)[0].(string)
	update := resp["sync"].(map[string]any)["upd"].(map[string]any)
	card := update["cards"].(map[string]any)[id].(map[string]any)["_db"].(map[string]any)
	potential := card["nvalue"].(map[string]any)
	for _, attribute := range []string{"hp", "damage", "defence", "speed", "specialDamage", "specialDefence"} {
		if _, ok := potential[attribute].(int); !ok {
			t.Errorf("nvalue[%q] is missing or not an integer: %v", attribute, potential)
		}
	}
}
