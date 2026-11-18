package game

import (
	"maps"
	"strconv"
)

func (p *Service) itemCount(itemID int64) int64 {
	return toInt64(p.items[itemID], 0)
}

func (p *Service) addItem(itemID, delta int64) {
	if delta == 0 {
		return
	}
	next := p.itemCount(itemID) + delta
	if next <= 0 {
		delete(p.items, itemID)
		return
	}
	p.items[itemID] = next
}

func (p *Service) fragCount(fragID int64) int64 {
	return toInt64(p.frags[fragID], 0)
}

func (p *Service) addFrag(fragID, delta int64) {
	if delta == 0 {
		return
	}
	next := p.fragCount(fragID) + delta
	if next <= 0 {
		delete(p.frags, fragID)
		return
	}
	p.frags[fragID] = next
}

func parseIDCountMap(raw any) (map[int64]int64, bool) {
	out := make(map[int64]int64)
	put := func(key, value any) bool {
		id := toInt64(key, 0)
		if str, ok := key.(string); ok {
			id, _ = strconv.ParseInt(str, 10, 64)
		}
		count := toInt64(value, 0)
		if id <= 0 || count <= 0 || count > 1000000 || out[id] != 0 {
			return false
		}
		out[id] = count
		return true
	}
	switch values := raw.(type) {
	case map[string]any:
		for key, value := range values {
			if !put(key, value) {
				return nil, false
			}
		}
	case map[any]any:
		for key, value := range values {
			if !put(key, value) {
				return nil, false
			}
		}
	case map[int64]any:
		for key, value := range values {
			if !put(key, value) {
				return nil, false
			}
		}
	}
	return out, len(out) > 0
}

func (p *Service) useItems(req any) map[string]any {
	inp := getInputMap(req)
	itemsD, ok := parseIDCountMap(inp["itemsD"])
	if !ok {
		return gameError("invalid_item")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for id, cnt := range itemsD {
		if p.itemCount(id) < cnt {
			return gameError("item_not_enough")
		}
	}
	var totalCount int64
	for id, cnt := range itemsD {
		p.addItem(id, -cnt)
		totalCount += cnt
	}
	p.gold += 500 * totalCount
	p.syncVersion++

	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"award": map[string]any{"gold": 500 * totalCount},
		},
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"items": maps.Clone(p.items),
						"gold":  p.gold,
					},
				},
			},
		},
	}
}

func (p *Service) sellItems(req any) map[string]any {
	inp := getInputMap(req)
	itemsD, ok := parseIDCountMap(inp["itemsD"])
	if !ok {
		return gameError("invalid_item")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for id, cnt := range itemsD {
		if p.itemCount(id) < cnt {
			return gameError("item_not_enough")
		}
	}
	var totalCount int64
	for id, cnt := range itemsD {
		p.addItem(id, -cnt)
		totalCount += cnt
	}
	p.gold += 100 * totalCount
	p.syncVersion++

	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"award": map[string]any{"gold": 100 * totalCount},
		},
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"items": maps.Clone(p.items),
						"gold":  p.gold,
					},
				},
			},
		},
	}
}

func (p *Service) sellFrags(req any) map[string]any {
	frags, ok := parseIDCountMap(getInputMap(req)["fragsD"])
	if !ok {
		return gameError("invalid_frag")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var coins int64
	for id, count := range frags {
		recipe := fragmentRecipes[id]
		if recipe.coins <= 0 {
			return gameError("invalid_frag")
		}
		if p.fragCount(id) < count {
			return gameError("frag_not_enough")
		}
		coins += recipe.coins * count
	}
	for id, count := range frags {
		p.addFrag(id, -count)
	}
	p.fragmentCoins += coins
	p.syncVersion++
	reply := p.roleSync(map[string]any{"frags": maps.Clone(p.frags), "coin5": p.fragmentCoins})
	replaceRoleFields(reply, "frags")
	reply["view"] = map[string]any{"coin5": coins}
	return reply
}

func (p *Service) combineFrag(req any) map[string]any {
	id := toInt64(getInputMap(req)["fragID"], 0)
	recipe, exists := fragmentRecipes[id]
	if !exists || recipe.card <= 0 || recipe.count <= 0 {
		return gameError("invalid_frag")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.cards) >= 100 {
		return gameError("card_capacity_full")
	}
	if p.fragCount(id) < recipe.count {
		return gameError("frag_not_enough")
	}
	seen := p.pokedexSnapshot()
	p.addFrag(id, -recipe.count)
	dbID := p.allocateCardID()
	card := newCard(dbID, recipe.card, 1)
	p.cards[dbID] = card
	p.registerSpecies(recipe.card)
	p.syncVersion++

	reply := p.roleSync(map[string]any{"cards": p.cardIDs(), "frags": maps.Clone(p.frags),
		"pokedex": p.pokedexSnapshot(),
	})
	replaceRoleFields(reply, "cards", "frags")
	reply["sync"].(map[string]any)["upd"].(map[string]any)["cards"] = map[string]any{dbID: snapshotModel(card)}
	reply["view"] = map[string]any{"db_id": dbID, "carddbIDs": []any{[]any{dbID, seen[recipe.card] == nil}}}
	return reply
}
func (p *Service) useStaminaItem(req any) map[string]any {
	inp := getInputMap(req)
	itemID := toInt64(inp["itemID"], 0)
	count := toInt64(inp["itemCount"], 1)
	if itemID <= 0 || count <= 0 {
		return gameError("invalid_item")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.itemCount(itemID) < count {
		return gameError("item_not_enough")
	}
	p.addItem(itemID, -count)
	p.stamina += 60 * count
	p.syncVersion++

	return p.roleSync(map[string]any{
		"items":   maps.Clone(p.items),
		"stamina": p.stamina,
	})
}

func (p *Service) buyStamina(_ any) map[string]any {
	const costRMB int64 = 50
	const gainStamina int64 = 120

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.rmb < costRMB {
		return gameError("rmb_not_enough")
	}
	p.rmb -= costRMB
	p.stamina += gainStamina
	p.buyStaminaTimes++
	p.syncVersion++

	return map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"rmb":     p.rmb,
						"stamina": p.stamina,
					},
				},
				"daily_record": map[string]any{
					"_db": map[string]any{
						"buy_stamina_times": p.buyStaminaTimes,
					},
				},
			},
		},
	}
}

func (p *Service) lianjin(req any) map[string]any {
	inp := getInputMap(req)
	count := toInt64(inp["count"], 1)
	if count <= 0 {
		count = 1
	}
	costRMB := 10 * count
	gainGold := 20000 * count

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.rmb < costRMB {
		return gameError("rmb_not_enough")
	}
	p.rmb -= costRMB
	p.gold += gainGold
	p.lianjinTimes += count
	p.syncVersion++

	results := make([]any, 0, count)
	for i := int64(0); i < count; i++ {
		results = append(results, map[string]any{
			"gold":     int64(20000),
			"multiple": int64(1),
		})
	}

	return map[string]any{
		"ret":  true,
		"view": results,
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"rmb":  p.rmb,
						"gold": p.gold,
					},
				},
				"daily_record": map[string]any{
					"_db": map[string]any{
						"lianjin_times": p.lianjinTimes,
					},
				},
			},
		},
	}
}

func (p *Service) lianjinGiftsSnapshot() map[int64]any {
	out := make(map[int64]any, len(p.lianjinGifts))
	for k, v := range p.lianjinGifts {
		out[k] = v
	}
	return out
}

func (p *Service) lianjinTotalAward(req any) map[string]any {
	inp := getInputMap(req)
	times := toInt64(inp["times"], 0)
	if times <= 0 {
		return gameError("invalid_params")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lianjinGifts == nil {
		p.lianjinGifts = make(map[int64]int64)
	}
	if p.lianjinTimes < times {
		return gameError("lianjin_times_not_enough")
	}
	if p.lianjinGifts[times] > 0 {
		return gameError("already_claimed")
	}
	gainGold := int64(30000)
	p.lianjinGifts[times] = 1
	p.gold += gainGold
	p.syncVersion++
	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"award": map[string]any{"gold": gainGold},
		},
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"gold": p.gold,
					},
				},
				"daily_record": map[string]any{
					"_db": map[string]any{
						"lianjin_gifts": p.lianjinGiftsSnapshot(),
					},
				},
			},
		},
	}
}

func (p *Service) renameRole(req any) map[string]any {
	inp := getInputMap(req)
	name, _ := inp["name"].(string)
	if name == "" {
		return gameError("name_can_not_empty")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.name = name
	p.syncVersion++
	return p.roleSync(map[string]any{"name": p.name})
}

func (p *Service) switchFigure(req any) map[string]any {
	inp := getInputMap(req)
	figure := toInt64(inp["figure"], 0)
	if figure <= 0 {
		return gameError("invalid_figure")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.figure = figure
	p.syncVersion++
	return p.roleSync(map[string]any{"figure": p.figure})
}
