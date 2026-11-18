package game

import (
	"cmp"
	"slices"
)

type rankedCard struct {
	id, level, skin, power int64
}

// rankMetrics is called with the player lock held.
func (p *Service) rankMetrics() (int64, []any, int64) {
	cards := make([]rankedCard, 0, len(p.cards))
	species := make(map[int64]bool)
	for _, raw := range p.cards {
		obj, _ := raw.(map[string]any)
		db, _ := obj["_db"].(map[string]any)
		card := rankedCard{
			id: toInt64(db["card_id"], 0), level: toInt64(db["level"], 1),
			skin: toInt64(db["skin_id"], 0), power: toInt64(db["fighting_point"], 0),
		}
		if card.id > 0 {
			cards = append(cards, card)
			species[card.id] = true
		}
	}
	slices.SortFunc(cards, func(a, b rankedCard) int {
		return cmp.Or(cmp.Compare(b.power, a.power), cmp.Compare(a.id, b.id),
			cmp.Compare(a.skin, b.skin), cmp.Compare(a.level, b.level))
	})
	var power int64
	top := make([]any, 0, min(6, len(cards)))
	for _, card := range cards[:min(6, len(cards))] {
		power += card.power
		top = append(top, map[string]any{"card_id": card.id, "level": card.level, "skin_id": card.skin})
	}
	return power, top, int64(len(species))
}

func (p *Service) localRank(req any) map[string]any {
	input := getInputMap(req)
	kind, _ := input["type"].(string)
	offset := toInt64(input["offest"], 0) // Client protocol spells this field "offest".
	size := toInt64(input["size"], 10)
	if offset < 0 || size < 0 {
		return gameError("invalid_rank_page")
	}
	rows := []any{}
	if offset == 0 && size > 0 {
		p.mu.Lock()
		defer p.mu.Unlock()
		power, top, collected := p.rankMetrics()
		entry := map[string]any{
			"role": map[string]any{
				"id": "100001", "uid": 100001, "name": p.name, "level": p.level, "logo": 1, "frame": 1,
			},
			"index": 1,
		}
		switch kind {
		case "fight":
			entry["fighting_point"], entry["top6_cards"] = power, top
			rows = append(rows, entry)
		case "pokedex":
			entry["pokedex"] = collected
			rows = append(rows, entry)
		case "star":
			var stars int64
			for _, gate := range p.gateStar {
				stars += gate.stars
			}
			entry["star"] = stars
			rows = append(rows, entry)
		}
	}
	return listView("rank", rows)
}

func listView(key string, rows []any) map[string]any {
	return map[string]any{"ret": true, "view": map[string]any{key: rows}}
}
