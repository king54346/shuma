package game

import (
	"maps"
	"strconv"
	"strings"
	"time"
)

func defaultCardEquips() map[int64]any {
	return map[int64]any{
		1: map[string]any{"equip_id": int64(11021), "level": int64(1), "advance": int64(1), "star": int64(0), "awake": int64(0), "awake_ability": int64(0), "ability": int64(0), "exp": int64(0), "signet": int64(0), "signet_advance": int64(0)},
		2: map[string]any{"equip_id": int64(11022), "level": int64(1), "advance": int64(1), "star": int64(0), "awake": int64(0), "awake_ability": int64(0), "ability": int64(0), "exp": int64(0), "signet": int64(0), "signet_advance": int64(0)},
		3: map[string]any{"equip_id": int64(11023), "level": int64(1), "advance": int64(1), "star": int64(0), "awake": int64(0), "awake_ability": int64(0), "ability": int64(0), "exp": int64(0), "signet": int64(0), "signet_advance": int64(0)},
		4: map[string]any{"equip_id": int64(11024), "level": int64(1), "advance": int64(1), "star": int64(0), "awake": int64(0), "awake_ability": int64(0), "ability": int64(0), "exp": int64(0), "signet": int64(0), "signet_advance": int64(0)},
	}
}

func newCard(id string, cardID, character int64) map[string]any {
	db := map[string]any{
		"id":             id,
		"card_id":        cardID,
		"unit_id":        cardID,
		"name":           "",
		"level":          1,
		"star":           1,
		"getstar":        1,
		"advance":        1,
		"exp":            0,
		"level_exp":      0,
		"next_level_exp": 100,
		"fighting_point": 500,
		"skills":         map[int64]any{},
		"equips":         defaultCardEquips(),
		"nvalue": map[string]any{
			"speed": 12, "specialDamage": 12, "damage": 12,
			"hp": 12, "specialDefence": 12, "defence": 12,
		},
		"nvalue_locked":       map[string]any{},
		"nvalue_recast_total": 0,
		"badge_guard":         []any{},
		"attrs":               map[string]any{},
		"attrs2":              map[string]any{},
		"effort_advance":      1,
		"fetters":             []any{},
		"ability_value":       map[string]any{},
		"effort_values":       map[string]any{},
		"feel_value":          0,
		"locked":              false,
		"character":           character,
		"skin_id":             0,
	}
	recalcCardAttrs(db)
	return map[string]any{"_db": db}
}

// allocateCardID is called with p.mu held.
func (p *Service) allocateCardID() string {
	id := strconv.FormatInt(p.nextCardID, 10)
	p.nextCardID++
	return id
}

type drawState struct {
	day, freeDiamond, freeGold, lastFreeGold, halfDiamond, dailyTotal int64
}

func (p *Service) drawDaily(now time.Time) {
	d := now.In(signZone)
	day := int64(d.Year()*10000 + int(d.Month())*100 + d.Day())
	if p.draws.day != day {
		p.draws = drawState{day: day}
	}
}

func (p *Service) drawSnapshot() map[string]any {
	return map[string]any{
		"dc1_free_count": p.draws.freeDiamond, "gold1_free_count": p.draws.freeGold,
		"gold1_free_last_time": p.draws.lastFreeGold, "draw_card_rmb1_half": p.draws.halfDiamond,
		"draw_card": p.draws.dailyTotal,
	}
}

func (p *Service) registerSpecies(id int64) {
	if p.registeredSpecies == nil {
		p.registeredSpecies = make(map[int64]int64)
	}
	if p.registeredSpecies[id] == 0 {
		p.registeredSpecies[id] = 1
	}
}

func (p *Service) pokedexSnapshot() map[int64]any {
	result := wireCounts(p.registeredSpecies)
	for _, raw := range p.cards {
		card, _ := raw.(map[string]any)
		db, _ := card["_db"].(map[string]any)
		if id := toInt64(db["card_id"], 0); id > 0 {
			result[id] = 1
		}
	}
	return result
}

func (p *Service) drawCards(req any) map[string]any {
	drawType, _ := getInputMap(req)["drawType"].(string)
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.drawDaily(now)

	count := 1
	if strings.HasSuffix(drawType, "10") {
		count = 10
	}
	if len(p.cards)+count > 500 {
		return gameError("card_capacity_full")
	}

	switch {
	case drawType == "free1":
		if p.draws.freeDiamond > 0 {
			return gameError("free_draw_unavailable")
		}
		p.draws.freeDiamond++
	case drawType == "free_gold1":
		if p.draws.freeGold >= 5 || now.Unix()-p.draws.lastFreeGold < 600 {
			return gameError("free_draw_unavailable")
		}
		p.draws.freeGold++
		p.draws.lastFreeGold = now.Unix()
	case strings.Contains(drawType, "free"):
		// Other pool free draws (e.g. splus_free1, limit_free1, eq_free1).
	case strings.Contains(drawType, "gold"):
		cost := int64(20000)
		if count == 10 {
			cost = 180000
		}
		if p.itemCount(518) >= int64(count) {
			p.addItem(518, -int64(count))
		} else if p.gold >= cost {
			p.gold -= cost
		} else {
			return gameError("gold_not_enough")
		}
	default:
		// All RMB / limited / splus / group_up / equip diamond draws.
		cost := int64(200)
		switch {
		case count == 10:
			cost = 1800
		case p.draws.halfDiamond == 0:
			cost = 100
		}
		useTicket := (count == 10 || p.draws.halfDiamond > 0) && p.itemCount(519) >= int64(count)
		if useTicket {
			p.addItem(519, -int64(count))
		} else if p.rmb >= cost {
			p.rmb -= cost
		} else {
			return gameError("rmb_not_enough")
		}
		if count == 1 {
			p.draws.halfDiamond = 1
		}
	}

	// Full diamond draw pool matching client config.draw_preview.
	normalPool := [...]int64{
		101, 1071, 11, 111, 1171, 121, 1221, 1231, 1241, 1251, 1291, 131, 1351, 1391,
		1421, 1441, 1501, 151, 161, 1611, 171, 1801, 1811, 1841, 1861, 191, 1911, 1921,
		1931, 2011, 2031, 2071, 21, 221, 2271, 2301, 231, 241, 2471, 2481, 2611, 2631,
		2681, 271, 2721, 281, 2841, 2871, 2891, 2921, 2922, 2961, 2991, 301, 3001, 3011,
		3021, 31, 3141, 3151, 3281, 3291, 331, 3341, 3351, 341, 3411, 3431, 3471, 3501,
		351, 3511, 3541, 3561, 3591, 3661, 371, 391, 41, 401, 451, 461, 471, 481, 51,
		541, 551, 571, 591, 601, 61, 631, 691, 71, 721, 731, 741, 781, 791, 801, 81,
		881, 891, 91, 1, 261, 491, 1081, 1311, 1851, 2591, 1261, 1971,
	}
	// Guaranteed S / S+ pool for 10-draw last slot (or UP selection).
	highTierPool := [...]int64{
		1221, 1231, 1911, 1921, 1931, 2471, 2481, 2611, 2631, 1261, 1971,
		721, 731, 741, 1171, 1241, 1251, 1291, 1391, 1611, 1801, 1811, 1841, 2071, 691, 781,
	}
	choose := toInt64(getInputMap(req)["choose"], 0)
	if choose <= 0 {
		choose = p.drawUpChoose
	}
	newCards := make(map[string]any, count)
	carddbIDs := make([]any, 0, count)
	seen := p.pokedexSnapshot()
	for i := 0; i < count; i++ {
		dbID := p.allocateCardID()
		var cid int64
		if count == 10 && i == count-1 {
			if choose > 0 {
				cid = choose
			} else {
				cid = highTierPool[int(p.draws.dailyTotal+int64(i))%len(highTierPool)]
			}
		} else if choose > 0 && i == 0 && (p.draws.dailyTotal%5 == 0) {
			cid = choose
		} else {
			idx := (int(p.draws.dailyTotal)*7 + i*13 + int(now.UnixNano()%int64(len(normalPool)))) % len(normalPool)
			if idx < 0 {
				idx = -idx
			}
			cid = normalPool[idx]
		}
		cObj := newCard(dbID, cid, 1)
		p.cards[dbID] = cObj
		p.registerSpecies(cid)
		newCards[dbID] = snapshotModel(cObj)
		carddbIDs = append(carddbIDs, []any{dbID, seen[cid] == nil})
		seen[cid] = 1
	}
	p.draws.dailyTotal += int64(count)
	p.syncVersion++
	reply := p.roleSync(map[string]any{
		"cards": p.cardIDs(), "pokedex": seen, "items": maps.Clone(p.items),
		"gold": p.gold, "rmb": p.rmb,
	})
	replaceRoleFields(reply, "cards", "items")
	upd := reply["sync"].(map[string]any)["upd"].(map[string]any)
	upd["cards"] = newCards
	upd["daily_record"] = map[string]any{"_db": p.drawSnapshot()}
	reply["view"] = map[string]any{"result": map[string]any{"carddbIDs": carddbIDs}}
	return reply
}

func (p *Service) chooseDrawUp(req any) map[string]any {
	choose := toInt64(getInputMap(req)["choose"], 0)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.drawUpChoose = choose
	p.syncVersion++
	return map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"lottery_record": map[string]any{
					"_db": map[string]any{
						"draw_card_up_choose": p.drawUpChoose,
					},
				},
			},
		},
	}
}

// Snapshot card models before a later request can update their nested attributes.
func snapshotModel(value any) any {
	switch v := value.(type) {
	case map[string]any:
		clone := make(map[string]any, len(v))
		for key, child := range v {
			clone[key] = snapshotModel(child)
		}
		return clone
	case map[int64]any:
		clone := make(map[int64]any, len(v))
		for key, child := range v {
			clone[key] = snapshotModel(child)
		}
		return clone
	case []any:
		clone := make([]any, len(v))
		for i, child := range v {
			clone[i] = snapshotModel(child)
		}
		return clone
	default:
		return value
	}
}

func replaceRoleFields(reply map[string]any, fields ...string) {
	db := make(map[string]any, len(fields))
	for _, field := range fields {
		db[field] = true
	}
	syncData := reply["sync"].(map[string]any)
	newFields, ok := syncData["new"].(map[string]any)
	if !ok {
		newFields = make(map[string]any)
		syncData["new"] = newFields
	}
	newFields["role"] = map[string]any{"_db": db}
}
