package game

import (
	"crypto/rand"
	"fmt"
	"maps"
	randv2 "math/rand/v2"
	"slices"
	"strconv"
	"time"
)

type readyTeam struct {
	Name  string
	Cards []string
}

func (p *Service) readyCardsSnapshot() map[int64]any {
	out := make(map[int64]any, len(p.readyCards))
	for idx, team := range p.readyCards {
		cards := make([]any, len(team.Cards))
		for i, id := range team.Cards {
			cards[i] = id
		}
		out[idx] = map[string]any{
			"name":  team.Name,
			"cards": cards,
		}
	}
	return out
}

func (p *Service) cardDeploymentSnapshot() map[string]any {
	arenaDef := make([]any, len(p.arenaDefenceCards))
	for i, id := range p.arenaDefenceCards {
		arenaDef[i] = id
	}
	return map[string]any{
		"union_training":    map[string]any{"cards": []any{}},
		"arena":             map[string]any{"defence_cards": arenaDef},
		"craft":             map[string]any{"cards": []any{}},
		"union_fight":       map[string]any{},
		"cross_union_fight": map[string]any{},
		"cross_craft":       map[string]any{"cards": []any{}},
		"cross_arena":       map[string]any{"defence_cards": []any{}},
		"gym":               map[string]any{"cards": []any{}, "cross_cards": []any{}},
		"cross_mine":        map[string]any{"defence_cards": []any{}},
		"cross_supremacy":   map[string]any{"cards": []any{}, "defence_cards": []any{}, "elite_cards": []any{}},
	}
}

// parseCardIDList extracts ordered card DB IDs from either a dense slice or a sparse 1-based slot map.
func parseCardIDList(raw any) ([]string, []any) {
	var ids []string
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if id, ok := item.(string); ok && id != "" {
				ids = append(ids, id)
			}
		}
	case map[any]any:
		type slot struct {
			idx int64
			id  string
		}
		var slots []slot
		for k, item := range v {
			if id, ok := item.(string); ok && id != "" {
				slots = append(slots, slot{idx: toInt64(k, 99), id: id})
			}
		}
		slices.SortFunc(slots, func(a, b slot) int { return int(a.idx - b.idx) })
		for _, s := range slots {
			ids = append(ids, s.id)
		}
	case map[string]any:
		type slot struct {
			idx int64
			id  string
		}
		var slots []slot
		for k, item := range v {
			if id, ok := item.(string); ok && id != "" {
				n, err := strconv.ParseInt(k, 10, 64)
				if err != nil {
					n = 99
				}
				slots = append(slots, slot{idx: n, id: id})
			}
		}
		slices.SortFunc(slots, func(a, b slot) int { return int(a.idx - b.idx) })
		for _, s := range slots {
			ids = append(ids, s.id)
		}
	}
	anyIDs := make([]any, len(ids))
	for i, id := range ids {
		anyIDs[i] = id
	}
	return ids, anyIDs
}

func (p *Service) setBattleCards(req any) map[string]any {
	inp := getInputMap(req)
	p.mu.Lock()
	defer p.mu.Unlock()
	newBattle, newBattleAny := parseCardIDList(inp["battleCardIDs"])
	if !p.validLineup(newBattle) {
		return gameError("invalid_battle_cards")
	}
	p.battleCards = newBattle
	p.syncVersion++
	reply := p.roleSync(map[string]any{"battle_cards": newBattleAny})
	replaceRoleFields(reply, "battle_cards")
	return reply
}

func (p *Service) validLineup(ids []string) bool {
	if len(ids) > 6 {
		return false
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if p.cards[id] == nil || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func (p *Service) deployReadyCards(req any) map[string]any {
	inp := getInputMap(req)
	idx := toInt64(inp["idx"], 0)
	if idx <= 0 {
		return gameError("invalid_ready_idx")
	}
	cards, _ := parseCardIDList(inp["cardIDs"])

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.validLineup(cards) {
		return gameError("invalid_battle_cards")
	}
	team := p.readyCards[idx]
	team.Cards = cards
	p.readyCards[idx] = team
	p.syncVersion++

	reply := p.roleSync(map[string]any{
		"ready_cards": p.readyCardsSnapshot(),
	})
	replaceRoleFields(reply, "ready_cards")
	return reply
}

func (p *Service) deployArenaCards(req any) map[string]any {
	inp := getInputMap(req)
	p.mu.Lock()
	defer p.mu.Unlock()

	raw := inp["defenceCards"]
	if raw == nil {
		raw = inp["cards"]
	}
	if raw != nil {
		ids, _ := parseCardIDList(raw)
		if !p.validLineup(ids) {
			return gameError("invalid_battle_cards")
		}
		p.arenaDefenceCards = ids
	}
	p.syncVersion++
	reply := p.roleSync(map[string]any{
		"card_deployment": p.cardDeploymentSnapshot(),
	})
	replaceRoleFields(reply, "card_deployment")
	return reply
}

func (p *Service) deploymentsSyncSnapshot() map[string]any {
	out := make(map[string]any, len(p.deploymentsSync))
	for k, v := range p.deploymentsSync {
		out[k] = v
	}
	return out
}

func (p *Service) syncDeployment(req any) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.deploymentsSync == nil {
		p.deploymentsSync = make(map[string]bool)
	}
	input := getInputMap(req)
	if key, ok := input["key"].(string); ok && key != "" {
		flag, _ := input["flag"].(bool)
		p.deploymentsSync[key] = flag
	}
	p.syncVersion++
	return p.roleSync(map[string]any{
		"card_deployment":  p.cardDeploymentSnapshot(),
		"deployments_sync": p.deploymentsSyncSnapshot(),
	})
}

func (p *Service) buildBattleLineup(lineup []string) ([]any, map[string]any) {
	if len(lineup) == 0 {
		lineup = []string{"1001"}
	}
	cardIDs := make([]any, 0, len(lineup))
	cardAttrs := make(map[string]any, len(lineup))
	for _, card := range lineup {
		obj, ok := p.cards[card].(map[string]any)
		if !ok {
			continue
		}
		db, ok := obj["_db"].(map[string]any)
		if !ok {
			continue
		}
		cardID := toInt64(db["card_id"], 21)
		skinID := toInt64(db["skin_id"], 0)
		fp := toInt64(db["fighting_point"], 500)
		lv := toInt64(db["level"], 1)
		adv := toInt64(db["advance"], 1)
		star := toInt64(db["star"], 1)

		skillsCopy := map[int64]any{}
		switch rawSkills := db["skills"].(type) {
		case map[int64]any:
			for sk, sv := range rawSkills {
				skillsCopy[sk] = sv
			}
		case map[string]any:
			for sk, sv := range rawSkills {
				if sid, err := strconv.ParseInt(sk, 10, 64); err == nil {
					skillsCopy[sid] = sv
				}
			}
		}

		attrsCopy := map[string]any{
			"hp": 1000, "mp1": 100, "speed": 100,
			"damage": 200, "defence": 100,
			"specialDamage": 200, "specialDefence": 100,
		}
		if rawAttrs, ok := db["attrs"].(map[string]any); ok {
			for ak, av := range rawAttrs {
				attrsCopy[ak] = av
			}
		}
		attrsCopy["mp1"] = int64(100)

		cardIDs = append(cardIDs, card)
		cardAttrs[card] = map[string]any{
			"card_id": cardID, "skin_id": skinID, "fighting_point": fp,
			"level": lv, "advance": adv, "star": star,
			"skills": skillsCopy,
			"attrs":  attrsCopy,
		}
	}
	return cardIDs, cardAttrs
}

func (p *Service) huodongShow(_ any) map[string]any {
	day := int64(time.Now().Weekday())
	if day == 0 {
		day = 7
	}
	open := make(map[int64]any, 20)
	for id := int64(1); id <= 20; id++ {
		open[id] = int64(1)
	}
	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"day":  day,
			"open": open,
		},
	}
}

func (p *Service) huodongStart(req any) map[string]any {
	inp := getInputMap(req)
	gateID := toInt64(inp["gateID"], 0)
	if gateID <= 0 {
		return gameError("invalid_gate")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	lineup, _ := parseCardIDList(inp["battleCardIDs"])
	if len(lineup) == 0 {
		lineup = p.battleCards
	}
	cardIDs, cardAttrs := p.buildBattleLineup(lineup)
	if len(cardIDs) == 0 {
		return gameError("invalid_battle_cards")
	}
	id := fmt.Sprintf("hd_%d_%d", time.Now().UnixNano(), gateID)
	return map[string]any{
		"ret": true,
		"model": map[string]any{
			"battle": map[string]any{
				"id": id, "gate_id": gateID, "rand_seed": randv2.IntN(1 << 30),
				"level": p.level, "name": p.name, "logo": 1, "figure": p.figure,
				"role_db_id": "100001", "cards": cardIDs, "card_attrs": cardAttrs,
				"passive_skills": []any{},
			},
		},
	}
}

func (p *Service) huodongEnd(req any) map[string]any {
	inp := getInputMap(req)
	result, _ := inp["result"].(string)
	p.mu.Lock()
	defer p.mu.Unlock()
	fields := map[string]any{}
	award := map[string]any{}
	if result == "win" {
		p.gold += 5000
		p.addItem(11, 5)
		p.addItem(5000, 3)
		fields["gold"] = p.gold
		fields["items"] = maps.Clone(p.items)
		award["gold"] = int64(5000)
		award["items"] = []any{[]any{int64(11), int64(5)}, []any{int64(5000), int64(3)}}
	}
	p.syncVersion++
	reply := p.roleSync(fields)
	if _, ok := fields["items"]; ok {
		replaceRoleFields(reply, "items")
	}
	reply["view"] = map[string]any{
		"result": result,
		"award":  award,
	}
	return reply
}

func (p *Service) huodongSaodang(req any) map[string]any {
	inp := getInputMap(req)
	times := max(int64(1), toInt64(inp["times"], 1))
	p.mu.Lock()
	defer p.mu.Unlock()
	results := make([]any, 0, times)
	for i := int64(0); i < times; i++ {
		p.gold += 5000
		p.addItem(11, 5)
		p.addItem(5000, 3)
		results = append(results, map[string]any{
			"exp": int64(0),
			"items": map[any]any{
				"gold":      int64(5000),
				int64(11):   int64(5),
				int64(5000): int64(3),
			},
		})
	}
	p.syncVersion++
	reply := p.roleSync(map[string]any{
		"gold":  p.gold,
		"items": maps.Clone(p.items),
	})
	replaceRoleFields(reply, "items")
	reply["view"] = map[string]any{"result": results}
	return reply
}

type gateProgress struct {
	stars int64
	chest int64
}

type gateBattle struct {
	id      string
	gate    int64
	expires time.Time
	model   map[string]any
}

type gateSettlement struct {
	gate    int64
	result  string
	stars   int64
	expires time.Time
	view    map[string]any
}

const battleLifetime = 30 * time.Minute

func (p *Service) startGate(req any) map[string]any {
	gate := toInt64(getInputMap(req)["gateID"], 0)
	p.mu.Lock()
	defer p.mu.Unlock()

	if !isValidNormalGate(gate) || !slices.Contains(p.gateOpen, gate) {
		return gameError("gate_locked")
	}
	now := time.Now()
	if active := p.activeGate; active != nil && active.gate == gate && now.Before(active.expires) {
		return p.startGateReply(active)
	}
	if p.stamina < firstChapterStamina {
		return gameError("insufficient_stamina")
	}
	for id, settlement := range p.settled {
		if !now.Before(settlement.expires) {
			delete(p.settled, id)
		}
	}

	id := rand.Text()
	cardIDs, cardAttrs := p.buildBattleLineup(p.battleCards)
	if len(cardIDs) == 0 {
		return gameError("invalid_battle_cards")
	}

	p.stamina -= firstChapterStamina
	p.rollTaskDay(now)
	p.gateDaily++
	p.staminaAt = now.Unix()
	p.syncVersion++
	p.activeGate = &gateBattle{
		id: id, gate: gate, expires: now.Add(battleLifetime),
		model: map[string]any{
			"id": id, "gate_id": gate, "rand_seed": randv2.IntN(1 << 30),
			"level": p.level, "name": p.name, "logo": 1, "figure": p.figure,
			"role_db_id": "100001", "cards": cardIDs, "card_attrs": cardAttrs,
			"passive_skills": []any{},
		},
	}
	return p.startGateReply(p.activeGate)
}

func (p *Service) startGateReply(battle *gateBattle) map[string]any {
	reply := p.taskSync(map[string]any{"stamina": p.stamina, "stamina_last_recover_time": p.staminaAt})
	reply["model"] = map[string]any{"battle": battle.model}
	return reply
}

func (p *Service) endGate(req any) map[string]any {
	input := getInputMap(req)
	id, _ := input["battleID"].(string)
	gate := toInt64(input["gateID"], 0)
	result, _ := input["result"].(string)
	stars := toInt64(input["star"], -1)
	if result == "fail" {
		stars = 0
	} else if result != "win" || stars < 1 || stars > 3 {
		return gameError("invalid_battle_result")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	if settled, ok := p.settled[id]; ok && now.Before(settled.expires) {
		if settled.gate != gate || settled.result != result || settled.stars != stars {
			return gameError("battle_result_conflict")
		}
		return p.endGateReply(settled.view)
	}
	if b := p.activeGate; b == nil || b.id != id || b.gate != gate || !now.Before(b.expires) {
		return gameError("invalid_or_expired_battle")
	}

	exp := int64(0)
	award := map[string]any{}
	if result == "win" {
		exp = firstChapterExp
		p.addRoleExp(exp)
		p.gold += firstChapterGold
		p.addItem(11, 1)
		award["gold"] = firstChapterGold
		award["items"] = []any{[]any{int64(11), int64(1)}}
		progress, cleared := p.gateStar[gate]
		if !cleared {
			progress.chest = -1
			if _, hasChest := firstChapterChests[gate]; hasChest {
				progress.chest = 1
			}
		}
		progress.stars = max(progress.stars, stars)
		p.gateStar[gate] = progress
		if next := nextNormalGate(gate); next > 0 && !slices.Contains(p.gateOpen, next) {
			p.gateOpen = append(p.gateOpen, next)
		}

		if m, ok := req.(map[string]any); ok {
			syncReq, _ := m["sync"].(map[string]any)
			roleReq, _ := syncReq["role"].(map[string]any)
			if guide := toInt64(roleReq["guideID"], 0); guide > 0 {
				p.addGuideID(guide)
			}
		}
	}
	view := map[string]any{
		"result": result, "star": stars, "award": award, "role": map[string]any{"addExp": exp},
	}
	p.activeGate = nil
	p.settled[id] = gateSettlement{gate: gate, result: result, stars: stars, expires: now.Add(battleLifetime), view: view}
	p.syncVersion++
	return p.endGateReply(view)
}

func (p *Service) addRoleExp(exp int64) {
	p.levelExp += exp
	for p.level < int64(len(roleLevelExp)) && p.levelExp >= roleLevelExp[p.level-1] {
		p.levelExp -= roleLevelExp[p.level-1]
		p.level++
	}
}

func (p *Service) endGateReply(view map[string]any) map[string]any {
	fields := p.gateSnapshot()
	guides := make([]any, len(p.newbieGuide))
	for i, id := range p.newbieGuide {
		guides[i] = id
	}
	fields["newbie_guide"] = guides
	fields["gold"] = p.gold
	fields["items"] = maps.Clone(p.items)
	fields["level"] = p.level
	fields["level_exp"] = p.levelExp
	fields["stamina"] = p.stamina
	fields["stamina_last_recover_time"] = p.staminaAt
	reply := p.taskSync(fields)
	reply["view"] = view
	return reply
}

func (p *Service) gateSnapshot() map[string]any {
	stars := make(map[int64]any, len(p.gateStar))
	total := int64(0)
	for gate, progress := range p.gateStar {
		stars[gate] = map[string]any{"star": progress.stars, "chest": progress.chest}
		total += progress.stars
	}
	open := make([]any, 0, len(p.gateOpen)+2)
	chapters := []int64{firstChapter, 111, 211}
	has20101 := false
	has30101 := false
	for _, gate := range p.gateOpen {
		open = append(open, gate)
		if gate == 20101 {
			has20101 = true
		}
		if gate == 30101 {
			has30101 = true
		}
		if ch := chapterForGate(gate); ch > 0 && !slices.Contains(chapters, ch) {
			chapters = append(chapters, ch)
		}
	}
	if !has20101 {
		open = append(open, int64(20101))
	}
	if !has30101 {
		open = append(open, int64(30101))
	}
	slices.Sort(chapters)
	mapOpen := make([]any, len(chapters))
	for i, ch := range chapters {
		mapOpen[i] = ch
	}
	milestones := make(map[int64]any)
	for i, tier := range chapterStarRewards {
		if tier.minStars == 0 {
			continue
		}
		switch {
		case p.gateClaims[int64(i)]:
			milestones[int64(i+1)] = 2
		case total >= tier.minStars:
			milestones[int64(i+1)] = 1
		}
	}
	return map[string]any{
		"gate_star": stars, "gate_star_sum": total, "gate_open": open, "map_open": mapOpen,
		"map_star": map[int64]any{firstChapter: map[string]any{"star_award": milestones}},
	}
}

func (p *Service) sweepGate(req any) map[string]any {
	inp := getInputMap(req)
	gate := toInt64(inp["gateID"], 0)
	times := toInt64(inp["times"], 1)
	if times <= 0 {
		times = 1
	}
	if times > 50 {
		times = 50
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !isValidNormalGate(gate) || !slices.Contains(p.gateOpen, gate) {
		return gameError("gate_locked")
	}
	costStamina := firstChapterStamina * times
	if p.stamina < costStamina {
		return gameError("insufficient_stamina")
	}
	p.stamina -= costStamina

	results := make([]any, 0, times)
	for i := int64(0); i < times; i++ {
		p.addRoleExp(firstChapterExp)
		p.gold += firstChapterGold
		p.addItem(5000, 1)
		p.addItem(11, 1)
		results = append(results, map[string]any{
			"exp": firstChapterExp,
			"items": map[any]any{
				"gold":      firstChapterGold,
				int64(5000): int64(1),
				int64(11):   int64(1),
			},
		})
	}
	p.gold += 500
	p.syncVersion++

	reply := p.taskSync(map[string]any{
		"gold":                      p.gold,
		"items":                     maps.Clone(p.items),
		"level":                     p.level,
		"level_exp":                 p.levelExp,
		"stamina":                   p.stamina,
		"stamina_last_recover_time": p.staminaAt,
	})
	reply["view"] = map[string]any{
		"result": results,
		"extra":  map[string]any{"gold": int64(500)},
	}
	return reply
}

func (p *Service) claimGateChest(req any) map[string]any {
	input := getInputMap(req)
	gate := toInt64(input["gateID"], 0)
	kind, _ := input["type"].(string)
	reward, exists := firstChapterChests[gate]
	if !exists || kind != "chest" {
		return gameError("invalid_gate_reward")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	progress := p.gateStar[gate]
	if progress.stars == 0 {
		return gameError("gate_reward_locked")
	}
	alreadyClaimed := progress.chest == 0
	progress.chest = 0
	p.gateStar[gate] = progress
	return p.gateRewardReply(reward, alreadyClaimed)
}

func (p *Service) claimChapterStars(req any) map[string]any {
	input := getInputMap(req)
	tier := toInt64(input["awardLevel"], 0)
	if toInt64(input["mapID"], 0) != firstChapter || tier < 1 || tier >= int64(len(chapterStarRewards)) {
		return gameError("invalid_star_reward")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	cfg := chapterStarRewards[tier]
	total := int64(0)
	for _, progress := range p.gateStar {
		total += progress.stars
	}
	if total < cfg.minStars {
		return gameError("star_reward_locked")
	}
	alreadyClaimed := p.gateClaims[tier]
	p.gateClaims[tier] = true
	return p.gateRewardReply(cfg.reward, alreadyClaimed)
}

func (p *Service) gateRewardReply(reward gateReward, alreadyClaimed bool) map[string]any {
	fields := p.gateSnapshot()
	view := p.applyReward(reward, alreadyClaimed, fields)
	reply := p.roleSync(fields)
	reply["view"] = view
	return reply
}

func (p *Service) applyReward(reward gateReward, alreadyClaimed bool, fields map[string]any) map[string]any {
	if !alreadyClaimed {
		p.gold += reward.gold
		p.rmb += reward.rmb
		p.syncVersion++
	}
	view := make(map[string]any)
	if reward.gold > 0 {
		view["gold"] = reward.gold
	}
	if reward.rmb > 0 {
		view["rmb"] = reward.rmb
	}
	fields["gold"], fields["rmb"] = p.gold, p.rmb
	items := make([]any, 0, len(reward.items)+len(reward.frags))
	for _, group := range []struct {
		key       string
		counts    map[int64]int64
		inventory map[int64]any
	}{{"items", reward.items, p.items}, {"frags", reward.frags, p.frags}} {
		updated := make(map[int64]any, len(group.counts))
		for id, count := range group.counts {
			if !alreadyClaimed {
				group.inventory[id] = toInt64(group.inventory[id], 0) + count
			}
			updated[id] = group.inventory[id]
			items = append(items, []any{id, count})
		}
		fields[group.key] = updated
	}
	if len(items) > 0 {
		view["items"] = items
	}
	return view
}
