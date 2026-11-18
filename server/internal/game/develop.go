package game

import (
	"maps"
	randv2 "math/rand/v2"
	"slices"
	"strconv"
)

func (p *Service) getCardDB(cardID string) (map[string]any, map[string]any, bool) {
	raw, ok := p.cards[cardID]
	if !ok {
		return nil, nil, false
	}
	cardMap, ok := raw.(map[string]any)
	if !ok {
		return nil, nil, false
	}
	db, ok := cardMap["_db"].(map[string]any)
	if !ok {
		return nil, nil, false
	}
	return cardMap, db, true
}

func (p *Service) cardSync(cardID string, cardMap map[string]any, roleFields map[string]any) map[string]any {
	upd := map[string]any{
		"cards": map[string]any{
			cardID: snapshotModel(cardMap),
		},
	}
	if len(roleFields) > 0 {
		upd["role"] = map[string]any{"_db": roleFields}
	}
	reply := map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd":     upd,
		},
	}
	if _, ok := roleFields["items"]; ok {
		replaceRoleFields(reply, "items")
	}
	return reply
}

var cardExpItems = map[int64]int64{11: 100, 12: 300, 13: 1000, 14: 3000}

func (p *Service) cardExpUseItem(req any) map[string]any {
	input := getInputMap(req)
	cardID, _ := input["cardID"].(string)
	item := toInt64(input["itemID"], 0)
	count := toInt64(input["itemCount"], 1)
	return p.feedCard(cardID, map[int64]int64{item: count})
}

func (p *Service) cardExpUseItems(req any) map[string]any {
	input := getInputMap(req)
	cardID, _ := input["cardID"].(string)
	items, ok := parseIDCountMap(input["items"])
	if !ok {
		return gameError("invalid_item")
	}
	return p.feedCard(cardID, items)
}

func (p *Service) feedCard(cardID string, items map[int64]int64) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	cardMap, db, ok := p.getCardDB(cardID)
	if !ok {
		return gameError("card_not_found")
	}
	var experience int64
	for id, count := range items {
		value := cardExpItems[id]
		if value == 0 || count <= 0 || count > 1000000 {
			return gameError("invalid_item")
		}
		if p.itemCount(id) < count {
			return gameError("item_not_enough")
		}
		experience += value * count
	}
	for id, count := range items {
		p.addItem(id, -count)
	}
	applyCardExp(db, experience)
	p.syncVersion++
	return p.cardSync(cardID, cardMap, map[string]any{"items": maps.Clone(p.items)})
}
func applyCardExp(db map[string]any, addExp int64) {
	lv := toInt64(db["level"], 1)
	curExp := toInt64(db["level_exp"], 0) + addExp
	nextExp := toInt64(db["next_level_exp"], 100)
	if nextExp <= 0 {
		nextExp = 100
	}
	for curExp >= nextExp && lv < 100 {
		curExp -= nextExp
		lv++
		nextExp = lv * 100
	}
	db["level"] = lv
	db["level_exp"] = curExp
	db["exp"] = toInt64(db["exp"], 0) + addExp
	db["next_level_exp"] = nextExp
	recalcCardAttrs(db)
}

func recalcCardAttrs(db map[string]any) {
	lv := toInt64(db["level"], 1)
	adv := toInt64(db["advance"], 1)
	star := toInt64(db["star"], 1)
	effAdv := toInt64(db["effort_advance"], 1)

	nvalueMap, _ := db["nvalue"].(map[string]any)
	effortMap, _ := db["effort_values"].(map[string]any)
	nvalBonus := func(key string) int64 {
		if nvalueMap == nil {
			return 0
		}
		return toInt64(nvalueMap[key], 0)
	}
	effortBonus := func(key string) int64 {
		if effortMap == nil {
			return 0
		}
		return toInt64(effortMap[key], 0)
	}

	growth := (100 + (adv-1)*25 + (star-1)*35 + effAdv*15) * (100 + (lv-1)*18) / 100

	hp := int64(10*growth) + nvalBonus("hp")*12 + effortBonus("hp")*4
	speed := int64(growth) + nvalBonus("speed")*2 + effortBonus("speed")
	damage := int64(2*growth) + nvalBonus("damage")*4 + effortBonus("damage")*2
	defence := int64(growth) + nvalBonus("defence")*3 + effortBonus("defence")
	specialDamage := int64(2*growth) + nvalBonus("specialDamage")*4 + effortBonus("specialDamage")*2
	specialDefence := int64(growth) + nvalBonus("specialDefence")*3 + effortBonus("specialDefence")

	attrs, ok := db["attrs"].(map[string]any)
	if !ok {
		attrs = map[string]any{}
		db["attrs"] = attrs
	}
	attrs["hp"] = hp
	attrs["speed"] = speed
	attrs["damage"] = damage
	attrs["defence"] = defence
	attrs["specialDamage"] = specialDamage
	attrs["specialDefence"] = specialDefence

	attrs2, ok := db["attrs2"].(map[string]any)
	if !ok {
		attrs2 = map[string]any{}
		db["attrs2"] = attrs2
	}
	attrs2["strike"] = 200 + (lv-1)*5 + (star-1)*50 + (adv-1)*30
	attrs2["damageAdd"] = (lv-1)*3 + (adv-1)*20
	attrs2["strikeDamage"] = 1500 + (lv-1)*10 + (star-1)*100
	attrs2["damageSub"] = (lv-1)*2 + (star-1)*20
	attrs2["strikeResistance"] = (lv-1)*2 + (adv-1)*15
	attrs2["ultimateAdd"] = (lv-1)*3 + (star-1)*25
	attrs2["block"] = 100 + (lv-1)*3 + (adv-1)*20
	attrs2["ultimateSub"] = (lv-1)*2 + (star-1)*15
	attrs2["breakBlock"] = (lv-1)*3 + (adv-1)*20
	attrs2["cure"] = (lv - 1) * 2
	attrs2["blockPower"] = 500 + (lv-1)*5
	attrs2["pvpDamageAdd"] = (star - 1) * 20
	attrs2["controlPer"] = (lv - 1) * 2
	attrs2["pvpDamageSub"] = (star - 1) * 20
	attrs2["immuneControl"] = (lv - 1) * 2
	attrs2["physicalDamageAdd"] = (adv - 1) * 15
	attrs2["defenceIgnore"] = (lv - 1) * 2
	attrs2["physicalDamageSub"] = (adv - 1) * 15
	attrs2["specialDefenceIgnore"] = (lv - 1) * 2
	attrs2["specialDamageAdd"] = (adv - 1) * 15
	attrs2["suckBlood"] = (lv - 1) * 2
	attrs2["specialDamageSub"] = (adv - 1) * 15
	attrs2["rebound"] = (lv - 1) * 2

	db["fighting_point"] = hp/4 + damage + specialDamage + defence + specialDefence + speed + (lv-1)*30 + (star-1)*200 + (adv-1)*120
}

func (p *Service) cardAdvance(req any) map[string]any {
	inp := getInputMap(req)
	cardID, _ := inp["cardID"].(string)
	count := toInt64(inp["count"], 1)
	if cardID == "" || count <= 0 {
		return gameError("invalid_params")
	}
	costGold := 500 * count

	p.mu.Lock()
	defer p.mu.Unlock()

	cardMap, db, ok := p.getCardDB(cardID)
	if !ok {
		return gameError("card_not_found")
	}
	if p.gold < costGold {
		return gameError("gold_not_enough")
	}
	p.gold -= costGold
	adv := toInt64(db["advance"], 1) + count
	db["advance"] = adv
	db["fighting_point"] = toInt64(db["fighting_point"], 500) + 150*count
	recalcCardAttrs(db)
	p.syncVersion++

	return p.cardSync(cardID, cardMap, map[string]any{
		"gold": p.gold,
	})
}

func (p *Service) cardStarUp(req any) map[string]any {
	inp := getInputMap(req)
	cardID, _ := inp["cardID"].(string)
	if cardID == "" {
		return gameError("invalid_params")
	}

	var costIDs []string
	if rawList, ok := inp["costCardIDs"].([]any); ok {
		for _, item := range rawList {
			if id, ok := item.(string); ok && id != "" {
				costIDs = append(costIDs, id)
			}
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cardMap, db, ok := p.getCardDB(cardID)
	if !ok {
		return gameError("card_not_found")
	}
	for _, cid := range costIDs {
		if cid == cardID || slices.Contains(p.battleCards, cid) {
			return gameError("card_in_battle")
		}
		_, cdb, exists := p.getCardDB(cid)
		if !exists {
			return gameError("card_not_found")
		}
		if locked, _ := cdb["locked"].(bool); locked {
			return gameError("card_locked")
		}
	}

	for _, cid := range costIDs {
		delete(p.cards, cid)
	}
	star := toInt64(db["star"], 1) + 1
	db["star"] = star
	db["fighting_point"] = toInt64(db["fighting_point"], 500) + 250
	recalcCardAttrs(db)
	p.syncVersion++

	resp := map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"cards": p.cardIDs(),
					},
				},
				"cards": map[string]any{
					cardID: cardMap,
				},
			},
		},
	}
	if len(costIDs) > 0 {
		delCards := make(map[string]any, len(costIDs))
		for _, cid := range costIDs {
			delCards[cid] = false
		}
		syncMap := resp["sync"].(map[string]any)
		syncMap["del"] = map[string]any{
			"cards": delCards,
		}
		syncMap["new"] = map[string]any{
			"role": map[string]any{"_db": map[string]any{"cards": true}},
		}
	}
	return resp
}

func (p *Service) cardSkillLevelUp(req any) map[string]any {
	inp := getInputMap(req)
	cardID, _ := inp["cardID"].(string)
	skillID := toInt64(inp["skillID"], 0)
	addLevel := toInt64(inp["addLevel"], 1)
	if cardID == "" || skillID <= 0 || addLevel <= 0 {
		return gameError("invalid_params")
	}
	costGold := 1000 * addLevel

	p.mu.Lock()
	defer p.mu.Unlock()

	cardMap, db, ok := p.getCardDB(cardID)
	if !ok {
		return gameError("card_not_found")
	}
	if p.gold < costGold {
		return gameError("gold_not_enough")
	}
	p.gold -= costGold

	skills, ok := db["skills"].(map[int64]any)
	if !ok {
		skills = map[int64]any{}
		if oldSkills, ok := db["skills"].(map[string]any); ok {
			for k, v := range oldSkills {
				if sid, err := strconv.ParseInt(k, 10, 64); err == nil {
					skills[sid] = v
				}
			}
		}
		db["skills"] = skills
	}
	curLv := toInt64(skills[skillID], 1)
	skills[skillID] = curLv + addLevel
	db["fighting_point"] = toInt64(db["fighting_point"], 500) + 30*addLevel
	p.syncVersion++

	return p.cardSync(cardID, cardMap, map[string]any{
		"gold": p.gold,
	})
}

func (p *Service) cardLockSwitch(req any) map[string]any {
	inp := getInputMap(req)
	cardID, _ := inp["cardID"].(string)
	if cardID == "" {
		return gameError("invalid_params")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cardMap, db, ok := p.getCardDB(cardID)
	if !ok {
		return gameError("card_not_found")
	}
	locked, _ := db["locked"].(bool)
	db["locked"] = !locked
	p.syncVersion++

	return p.cardSync(cardID, cardMap, nil)
}

func (p *Service) equipUpgrade(req any, mode string) map[string]any {
	inp := getInputMap(req)
	cardID, _ := inp["cardID"].(string)
	equipPos := toInt64(inp["equipPos"], 1)
	if cardID == "" || equipPos <= 0 {
		return gameError("invalid_params")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cardMap, db, ok := p.getCardDB(cardID)
	if !ok {
		return gameError("card_not_found")
	}
	equips, ok := db["equips"].(map[int64]any)
	if !ok {
		equips = defaultCardEquips()
		db["equips"] = equips
	}
	eq, ok := equips[equipPos].(map[string]any)
	if !ok {
		eq = map[string]any{
			"equip_id": int64(11020) + equipPos,
			"level":    int64(1),
			"advance":  int64(1),
			"star":     int64(0),
			"awake":    int64(0),
			"ability":  int64(0),
			"exp":      int64(0),
		}
		equips[equipPos] = eq
	}

	switch mode {
	case "strength":
		up := toInt64(inp["upLevel"], 1)
		if up <= 0 {
			up = 1
		}
		eq["level"] = toInt64(eq["level"], 1) + up
		db["fighting_point"] = toInt64(db["fighting_point"], 500) + 20*up
	case "advance":
		eq["advance"] = toInt64(eq["advance"], 1) + 1
		db["fighting_point"] = toInt64(db["fighting_point"], 500) + 80
	case "star":
		eq["star"] = toInt64(eq["star"], 0) + 1
		db["fighting_point"] = toInt64(db["fighting_point"], 500) + 120
	}
	p.syncVersion++

	return p.cardSync(cardID, cardMap, nil)
}

var cardAttrKeys = []string{"hp", "speed", "damage", "defence", "specialDamage", "specialDefence"}

func (p *Service) cardNValueLockSwitch(req any) map[string]any {
	inp := getInputMap(req)
	cardID, _ := inp["cardID"].(string)
	attr, _ := inp["attr"].(string)
	if cardID == "" || attr == "" {
		return gameError("invalid_params")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cardMap, db, ok := p.getCardDB(cardID)
	if !ok {
		return gameError("card_not_found")
	}
	lockedMap, ok := db["nvalue_locked"].(map[string]any)
	if !ok {
		lockedMap = map[string]any{}
		db["nvalue_locked"] = lockedMap
	}
	cur, _ := lockedMap[attr].(bool)
	if cur {
		lockedMap[attr] = false
	} else {
		lockedMap[attr] = true
	}
	p.syncVersion++
	return p.cardSync(cardID, cardMap, nil)
}

func (p *Service) cardNValueRecast(req any) map[string]any {
	inp := getInputMap(req)
	cardID, _ := inp["cardID"].(string)
	if cardID == "" {
		return gameError("invalid_params")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cardMap, db, ok := p.getCardDB(cardID)
	if !ok {
		return gameError("card_not_found")
	}
	nvalue, ok := db["nvalue"].(map[string]any)
	if !ok {
		nvalue = map[string]any{}
		db["nvalue"] = nvalue
	}
	lockedMap, _ := db["nvalue_locked"].(map[string]any)
	for _, k := range cardAttrKeys {
		if locked, _ := lockedMap[k].(bool); locked {
			continue
		}
		nvalue[k] = int64(15 + randv2.IntN(17))
	}
	db["fighting_point"] = toInt64(db["fighting_point"], 500) + 25
	db["nvalue_recast_total"] = toInt64(db["nvalue_recast_total"], 0) + 1
	recalcCardAttrs(db)
	p.syncVersion++
	return p.cardSync(cardID, cardMap, nil)
}

func (p *Service) cardEffortTrain(req any) map[string]any {
	inp := getInputMap(req)
	cardID, _ := inp["cardID"].(string)
	trainTime := toInt64(inp["trainTime"], 1)
	if cardID == "" || trainTime <= 0 {
		return gameError("invalid_params")
	}
	costGold := 500 * trainTime

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, _, ok := p.getCardDB(cardID); !ok {
		return gameError("card_not_found")
	}
	if p.gold < costGold {
		return gameError("gold_not_enough")
	}
	p.gold -= costGold

	deltas := make(map[string]int64, len(cardAttrKeys))
	viewDeltas := make(map[string]any, len(cardAttrKeys))
	for _, k := range cardAttrKeys {
		d := int64(2+randv2.IntN(5)) * trainTime
		deltas[k] = d
		viewDeltas[k] = d
	}
	p.pendingEffort[cardID] = deltas
	p.syncVersion++

	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"result": []any{viewDeltas},
		},
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"gold": p.gold,
					},
				},
			},
		},
	}
}

func (p *Service) cardEffortSave(req any) map[string]any {
	inp := getInputMap(req)
	cardID, _ := inp["cardID"].(string)
	if cardID == "" {
		return gameError("invalid_params")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cardMap, db, ok := p.getCardDB(cardID)
	if !ok {
		return gameError("card_not_found")
	}
	eff, ok := db["effort_values"].(map[string]any)
	if !ok {
		eff = map[string]any{}
		db["effort_values"] = eff
	}
	deltas := p.pendingEffort[cardID]
	if len(deltas) == 0 {
		return gameError("no_pending_effort")
	}
	for k, d := range deltas {
		eff[k] = toInt64(eff[k], 0) + d
	}
	delete(p.pendingEffort, cardID)
	db["fighting_point"] = toInt64(db["fighting_point"], 500) + 40
	recalcCardAttrs(db)
	p.syncVersion++
	return p.cardSync(cardID, cardMap, nil)
}

func (p *Service) cardEffortAdvance(req any) map[string]any {
	inp := getInputMap(req)
	cardID, _ := inp["cardID"].(string)
	if cardID == "" {
		return gameError("invalid_params")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cardMap, db, ok := p.getCardDB(cardID)
	if !ok {
		return gameError("card_not_found")
	}
	db["effort_advance"] = toInt64(db["effort_advance"], 0) + 1
	db["fighting_point"] = toInt64(db["fighting_point"], 500) + 100
	recalcCardAttrs(db)
	p.syncVersion++
	return p.cardSync(cardID, cardMap, nil)
}

func (p *Service) cardNatureChoose(req any) map[string]any {
	inp := getInputMap(req)
	cardID, _ := inp["cardID"].(string)
	choose := toInt64(inp["choose"], 1)
	if cardID == "" {
		return gameError("invalid_params")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cardMap, db, ok := p.getCardDB(cardID)
	if !ok {
		return gameError("card_not_found")
	}
	db["nature_choose"] = choose
	p.syncVersion++
	return p.cardSync(cardID, cardMap, nil)
}

func (p *Service) cardDevelop(req any) map[string]any {
	inp := getInputMap(req)
	cardID, _ := inp["cardID"].(string)
	if cardID == "" {
		return gameError("invalid_params")
	}
	const costGold int64 = 1000

	p.mu.Lock()
	defer p.mu.Unlock()

	cardMap, db, ok := p.getCardDB(cardID)
	if !ok {
		return gameError("card_not_found")
	}
	if p.gold < costGold {
		return gameError("gold_not_enough")
	}
	p.gold -= costGold
	newCardID := toInt64(db["card_id"], 1) + 1
	db["card_id"] = newCardID
	db["unit_id"] = newCardID
	db["develop"] = toInt64(db["develop"], 1) + 1
	db["fighting_point"] = toInt64(db["fighting_point"], 500) + 200
	recalcCardAttrs(db)
	p.registerSpecies(newCardID)
	p.syncVersion++

	return p.cardSync(cardID, cardMap, map[string]any{
		"gold":    p.gold,
		"pokedex": p.pokedexSnapshot(),
	})
}
