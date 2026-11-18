package game

import (
	"maps"
	"slices"
	"strconv"

	"shuma-server/internal/playerstore"
)

var persistedCardFields = []string{"equips", "nvalue", "nvalue_locked", "effort_values", "effort_advance", "nature_choose"}

// NewWithStore initializes a Service backed by persistent playerstore storage.
func NewWithStore(store *playerstore.Store, key playerstore.AccountKey) (*Service, error) {
	save, err := store.LoadOrCreate(key)
	if err != nil {
		return nil, err
	}
	svc := New()
	svc.store = store
	svc.storeKey = key
	svc.applySave(save)
	return svc, nil
}

func (p *Service) applySave(save playerstore.PlayerSave) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.name = save.Role.Name
	p.figure = save.Role.Figure
	p.level = save.Role.Level
	p.levelExp = save.Role.LevelExp
	p.gold = save.Role.Gold
	p.rmb = save.Role.RMB
	p.stamina = save.Role.Stamina
	p.staminaAt = save.Role.StaminaLastRecoverTime
	p.syncVersion = save.SyncVersion
	p.savedVersion = save.SyncVersion
	p.frags = wireCounts(save.Fragments)
	p.fragmentCoins = save.Menu.FragmentCoins
	p.registeredSpecies = maps.Clone(save.Menu.Species)
	p.pokedexAdv = wireCounts(save.Menu.PokedexAdvance)
	p.draws = drawState{
		day: save.Menu.Draw.Day, freeDiamond: save.Menu.Draw.FreeDiamond,
		freeGold: save.Menu.Draw.FreeGold, lastFreeGold: save.Menu.Draw.LastFreeGold,
		halfDiamond: save.Menu.Draw.HalfDiamond, dailyTotal: save.Menu.Draw.DailyTotal,
	}
	for key, state := range save.Menu.Shops {
		if len(menuShops[key]) > 0 && state.Cycle >= 0 {
			p.shops[key] = shopState{Cycle: state.Cycle, Updated: state.Updated, Bought: maps.Clone(state.Bought)}
		}
	}
	for idx, team := range save.Menu.Ready {
		p.readyCards[idx] = readyTeam{Name: team.Name, Cards: append([]string(nil), team.Cards...)}
	}
	p.arenaDefenceCards = append([]string(nil), save.Menu.Defence...)
	p.taskDay, p.gateDaily, p.liveness = save.Menu.TaskDay, save.Menu.GateAttempts, save.Menu.Liveness
	maps.Copy(p.dailyTasks, save.Menu.DailyClaims)
	maps.Copy(p.mainTasksClaimed, save.Menu.MainClaims)
	p.livenessAwards = wireCounts(save.Menu.LivenessClaims)
	maps.Copy(p.guideAwards, save.GuideAwards)
	maps.Copy(p.gateClaims, save.ChapterClaims)
	p.sign = signState{
		month: save.Sign.Month, count: save.Sign.Count, lastDay: save.Sign.LastDay,
		lastIndex: save.Sign.LastIndex, buys: save.Sign.Buys,
		total: save.Sign.Total, claimed: save.Sign.Claimed,
		monthGifts: wireCounts(save.Sign.MonthGifts),
		awards:     make(map[int64]any, len(save.Sign.DailyAwards)),
	}
	for day, index := range save.Sign.DailyAwards {
		p.sign.awards[day] = map[int64]any{index: 0}
	}
	if len(save.NewbieGuide) > 0 {
		p.newbieGuide = append([]int64(nil), save.NewbieGuide...)
	}

	p.items = make(map[int64]any, len(save.Items))
	for k, v := range save.Items {
		p.items[k] = v
	}

	p.houseLovers = make(map[int64]lover, len(save.HouseLovers))
	for k, v := range save.HouseLovers {
		p.houseLovers[k] = lover{feel: v.Feel, card: v.Card}
	}

	if len(save.Cards) > 0 {
		p.cards = make(map[string]any, len(save.Cards))
		var maxID int64 = 1001
		for id, c := range save.Cards {
			if n, err := strconv.ParseInt(id, 10, 64); err == nil && n > maxID {
				maxID = n
			}
			skills := make(map[int64]any, len(c.Skills))
			for sk, sv := range c.Skills {
				skills[sk] = sv
			}
			cardObj := newCard(c.ID, c.CardID, c.Character)
			db := cardObj["_db"].(map[string]any)
			db["level"] = c.Level
			db["star"] = c.Star
			db["advance"] = c.Advance
			db["exp"] = c.Exp
			db["level_exp"] = c.LevelExp
			db["next_level_exp"] = c.NextLevelExp
			db["fighting_point"] = c.FightingPoint
			db["skills"] = skills
			db["locked"] = c.Locked
			db["skin_id"] = c.SkinID
			for _, field := range persistedCardFields {
				if v, ok := c.Extra[field]; ok {
					db[field] = snapshotModel(v)
				}
			}
			if toInt64(db["effort_advance"], 0) < 1 {
				db["effort_advance"] = int64(1)
			}
			if toInt64(db["unit_id"], 0) <= 0 {
				db["unit_id"] = c.CardID
			}
			if toInt64(db["getstar"], 0) < 1 {
				db["getstar"] = int64(1)
			}
			db["equips"] = normalizeCardEquips(db["equips"])
			recalcCardAttrs(db)
			p.cards[id] = cardObj
			p.registerSpecies(c.CardID)
		}
		p.nextCardID = maxID + 1
	}

	if save.BattleCards != nil {
		p.battleCards = append([]string(nil), save.BattleCards...)
	}

	p.gateStar = make(map[int64]gateProgress, len(save.GateStar))
	for k, v := range save.GateStar {
		p.gateStar[k] = gateProgress{stars: v.Star, chest: v.Chest}
	}
	if len(save.GateOpen) > 0 {
		p.gateOpen = append([]int64(nil), save.GateOpen...)
	}
	for gate, progress := range p.gateStar {
		if progress.stars > 0 {
			if next := nextNormalGate(gate); next > 0 && !slices.Contains(p.gateOpen, next) {
				p.gateOpen = append(p.gateOpen, next)
			}
		}
	}
}

func wireCounts(counts map[int64]int64) map[int64]any {
	values := make(map[int64]any, len(counts))
	for id, count := range counts {
		values[id] = count
	}
	return values
}

func saveCounts(values map[int64]any) map[int64]int64 {
	counts := make(map[int64]int64, len(values))
	for id, value := range values {
		counts[id] = toInt64(value, 0)
	}
	return counts
}

func ensureEquipDefaults(em map[string]any, pos int64) {
	if toInt64(em["equip_id"], 0) <= 0 {
		em["equip_id"] = int64(11020) + pos
	}
	for _, k := range []string{"star", "awake", "awake_ability", "ability", "exp", "signet", "signet_advance"} {
		if _, ok := em[k]; !ok {
			em[k] = int64(0)
		}
	}
	if toInt64(em["level"], 0) < 1 {
		em["level"] = int64(1)
	}
	if toInt64(em["advance"], 0) < 1 {
		em["advance"] = int64(1)
	}
}

func normalizeCardEquips(raw any) map[int64]any {
	out := defaultCardEquips()
	switch m := raw.(type) {
	case map[int64]any:
		for k, v := range m {
			if em, ok := v.(map[string]any); ok {
				ensureEquipDefaults(em, k)
				out[k] = em
			}
		}
	case map[string]any:
		for k, v := range m {
			pos, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				continue
			}
			if em, ok := v.(map[string]any); ok {
				ensureEquipDefaults(em, pos)
				out[pos] = em
			}
		}
	}
	return out
}

func (p *Service) persistIfChanged() error {
	p.mu.Lock()
	name := p.name
	figure := p.figure
	level := p.level
	levelExp := p.levelExp
	gold := p.gold
	rmb := p.rmb
	stamina := p.stamina
	staminaAt := p.staminaAt
	syncVer := p.syncVersion
	if syncVer == p.savedVersion {
		p.mu.Unlock()
		return nil
	}
	guides := append([]int64(nil), p.newbieGuide...)
	battleCards := append([]string(nil), p.battleCards...)
	gateOpen := append([]int64(nil), p.gateOpen...)

	items := saveCounts(p.items)
	frags := saveCounts(p.frags)
	guidesClaimed := maps.Clone(p.guideAwards)
	chapterClaims := maps.Clone(p.gateClaims)
	menu := playerstore.MenuRecord{
		Species: maps.Clone(p.registeredSpecies), PokedexAdvance: saveCounts(p.pokedexAdv),
		Draw: playerstore.DrawRecord{
			Day: p.draws.day, FreeDiamond: p.draws.freeDiamond, FreeGold: p.draws.freeGold,
			LastFreeGold: p.draws.lastFreeGold, HalfDiamond: p.draws.halfDiamond, DailyTotal: p.draws.dailyTotal,
		},
		Shops:   make(map[string]playerstore.ShopRecord, len(p.shops)),
		Ready:   make(map[int64]playerstore.TeamRecord, len(p.readyCards)),
		Defence: append([]string(nil), p.arenaDefenceCards...),
		TaskDay: p.taskDay, GateAttempts: p.gateDaily, Liveness: p.liveness,
		DailyClaims: maps.Clone(p.dailyTasks), MainClaims: maps.Clone(p.mainTasksClaimed),
		LivenessClaims: saveCounts(p.livenessAwards),
		FragmentCoins:  p.fragmentCoins,
	}
	for key, state := range p.shops {
		menu.Shops[key] = playerstore.ShopRecord{Cycle: state.Cycle, Updated: state.Updated, Bought: maps.Clone(state.Bought)}
	}
	for idx, team := range p.readyCards {
		menu.Ready[idx] = playerstore.TeamRecord{Name: team.Name, Cards: append([]string(nil), team.Cards...)}
	}
	sign := playerstore.SignRecord{
		Month: p.sign.month, Count: p.sign.count, LastDay: p.sign.lastDay,
		LastIndex: p.sign.lastIndex, Buys: p.sign.buys,
		Total: p.sign.total, Claimed: p.sign.claimed,
		MonthGifts:  saveCounts(p.sign.monthGifts),
		DailyAwards: make(map[int64]int64, len(p.sign.awards)),
	}
	for day, raw := range p.sign.awards {
		for index := range raw.(map[int64]any) {
			sign.DailyAwards[day] = index
		}
	}
	lovers := make(map[int64]playerstore.HouseLoverRecord, len(p.houseLovers))
	for k, v := range p.houseLovers {
		lovers[k] = playerstore.HouseLoverRecord{Feel: v.feel, Card: v.card}
	}
	gates := make(map[int64]playerstore.GateStarRecord, len(p.gateStar))
	for k, v := range p.gateStar {
		gates[k] = playerstore.GateStarRecord{Star: v.stars, Chest: v.chest}
	}
	cards := make(map[string]playerstore.CardRecord, len(p.cards))
	for id, raw := range p.cards {
		cMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		db, ok := cMap["_db"].(map[string]any)
		if !ok {
			continue
		}
		skills := map[int64]int64{}
		switch rawSkills := db["skills"].(type) {
		case map[int64]any:
			for sid, sv := range rawSkills {
				skills[sid] = toInt64(sv, 1)
			}
		case map[string]any:
			for sk, sv := range rawSkills {
				if sid, err := strconv.ParseInt(sk, 10, 64); err == nil {
					skills[sid] = toInt64(sv, 1)
				}
			}
		}
		locked, _ := db["locked"].(bool)
		extra := make(map[string]any)
		for _, field := range persistedCardFields {
			if v, ok := db[field]; ok {
				extra[field] = snapshotModel(v)
			}
		}
		cards[id] = playerstore.CardRecord{
			ID:            id,
			CardID:        toInt64(db["card_id"], 1),
			Level:         toInt64(db["level"], 1),
			Star:          toInt64(db["star"], 1),
			Advance:       toInt64(db["advance"], 1),
			Exp:           toInt64(db["exp"], 0),
			LevelExp:      toInt64(db["level_exp"], 0),
			NextLevelExp:  toInt64(db["next_level_exp"], 100),
			FightingPoint: toInt64(db["fighting_point"], 500),
			Skills:        skills,
			Locked:        locked,
			Character:     toInt64(db["character"], 1),
			SkinID:        toInt64(db["skin_id"], 0),
			Extra:         extra,
		}
	}
	p.mu.Unlock()

	_, err := p.store.Update(p.storeKey, func(s *playerstore.PlayerSave) error {
		s.Role.Name = name
		s.Role.Figure = figure
		s.Role.Level = level
		s.Role.LevelExp = levelExp
		s.Role.Gold = gold
		s.Role.RMB = rmb
		s.Role.Stamina = stamina
		s.Role.StaminaLastRecoverTime = staminaAt
		s.NewbieGuide = guides
		s.Items = items
		s.Fragments = frags
		s.GuideAwards = guidesClaimed
		s.ChapterClaims = chapterClaims
		s.Sign = sign
		s.Menu = menu
		s.HouseLovers = lovers
		s.Cards = cards
		s.BattleCards = battleCards
		s.GateStar = gates
		s.GateOpen = gateOpen
		if syncVer > s.SyncVersion {
			s.SyncVersion = syncVer - 1
		}
		return nil
	})
	if err == nil {
		p.savedVersion = syncVer
	}
	return err
}
