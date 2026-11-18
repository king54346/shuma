package game

import (
	"fmt"
	"maps"
	"time"
)

func (p *Service) arenaDefenceAttrsSnapshot() (map[string]any, int64) {
	ids := p.arenaDefenceCards
	if len(ids) == 0 {
		ids = p.battleCards
	}
	attrs := make(map[string]any, len(ids))
	var totalFP int64
	for _, id := range ids {
		if _, db, ok := p.getCardDB(id); ok {
			fp := toInt64(db["fighting_point"], 500)
			totalFP += fp
			attrs[id] = map[string]any{
				"id":             id,
				"card_id":        toInt64(db["card_id"], 21),
				"level":          toInt64(db["level"], 1),
				"star":           toInt64(db["star"], 1),
				"advance":        toInt64(db["advance"], 1),
				"fighting_point": fp,
			}
		}
	}
	if len(attrs) == 0 {
		attrs["1001"] = map[string]any{
			"id": "1001", "card_id": 21, "level": 1, "star": 1, "advance": 1, "fighting_point": 500,
		}
		totalFP = 500
	}
	return attrs, totalFP
}

func (p *Service) arenaRecordSnapshot() map[string]any {
	defIDs := p.arenaDefenceCards
	if len(defIDs) == 0 {
		defIDs = p.battleCards
	}
	defAny := make([]any, len(defIDs))
	for i, id := range defIDs {
		defAny[i] = id
	}
	atkAny := make([]any, len(p.battleCards))
	for i, id := range p.battleCards {
		atkAny[i] = id
	}
	defAttrs, totalFP := p.arenaDefenceAttrsSnapshot()
	return map[string]any{
		"rank":               p.arenaRank,
		"rank_top":           p.arenaRankTop,
		"history_rank":       p.arenaRankTop,
		"role_db_id":         "100001",
		"record_id":          100001,
		"role_name":          p.name,
		"role_level":         p.level,
		"role_logo":          1,
		"role_frame":         1,
		"role_figure":        p.figure,
		"display":            p.arenaDisplay,
		"fighting_point":     totalFP,
		"cards":              atkAny,
		"defence_cards":      defAny,
		"defence_card_attrs": defAttrs,
		"result_point":       p.arenaResultPoint,
		"combat_records":     []any{},
	}
}

func arenaBotEntry(rank, cardID, fp int64, name string) map[string]any {
	botCardID := fmt.Sprintf("bot_%d_1", rank)
	return map[string]any{
		"record_id":      90000 + rank,
		"role_db_id":     fmt.Sprintf("bot_%d", rank),
		"rank":           rank,
		"role_name":      name,
		"role_level":     max(1, 30-rank/5),
		"role_logo":      1,
		"role_frame":     1,
		"role_figure":    1,
		"display":        cardID,
		"fighting_point": fp,
		"defence_cards":  []any{botCardID},
		"defence_card_attrs": map[string]any{
			botCardID: map[string]any{
				"id":             botCardID,
				"card_id":        cardID,
				"level":          10,
				"star":           2,
				"advance":        2,
				"fighting_point": fp,
			},
		},
	}
}

func (p *Service) arenaTop10() []any {
	return []any{
		arenaBotEntry(1, 21, 9500, "冠军训练师"),
		arenaBotEntry(2, 11, 8600, "四天王·阿渡"),
		arenaBotEntry(3, 1, 7800, "道馆馆主·小茂"),
	}
}

func (p *Service) arenaEnemys() []any {
	r1 := max(1, p.arenaRank-15)
	r2 := max(1, p.arenaRank-8)
	r3 := max(1, p.arenaRank-2)
	return []any{
		arenaBotEntry(r1, 1, 1800, "挑战者·小智"),
		arenaBotEntry(r2, 11, 1400, "挑战者·小霞"),
		arenaBotEntry(r3, 21, 900, "挑战者·小刚"),
	}
}

func (p *Service) arenaModelSync() map[string]any {
	return map[string]any{
		"top10":  p.arenaTop10(),
		"enemys": p.arenaEnemys(),
		"record": p.arenaRecordSnapshot(),
	}
}

func (p *Service) pwBattleGet(req any) map[string]any {
	inp := getInputMap(req)
	needRefresh, _ := inp["needRefresh"].(bool)

	p.mu.Lock()
	defer p.mu.Unlock()

	if needRefresh {
		p.pvpRefreshTimes++
	}
	p.syncVersion++
	return map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"arena": p.arenaModelSync(),
				"daily_record": map[string]any{
					"_db": map[string]any{
						"pvp_enermys_refresh_times": p.pvpRefreshTimes,
					},
				},
			},
		},
	}
}

func (p *Service) pwBattleStart(req any) map[string]any {
	inp := getInputMap(req)
	battleRank := toInt64(inp["battleRank"], max(1, p.arenaRank-2))

	p.mu.Lock()
	defer p.mu.Unlock()

	cardIDs, cardAttrs := p.buildBattleLineup(p.battleCards)
	if len(cardIDs) == 0 {
		return gameError("invalid_battle_cards")
	}
	defID := "pw_bot_1"
	defAttrs := map[string]any{
		"hp": int64(1200), "mp1": int64(100), "speed": int64(100),
		"damage": int64(220), "defence": int64(110),
		"specialDamage": int64(220), "specialDefence": int64(110),
	}
	battleID := fmt.Sprintf("pw_%d_%d", time.Now().UnixNano(), battleRank)
	return map[string]any{
		"ret": true,
		"model": map[string]any{
			"battle": map[string]any{
				"id":                     battleID,
				"gate_id":                int64(0),
				"rand_seed":              time.Now().Unix() & 0x7fffffff,
				"level":                  p.level,
				"name":                   p.name,
				"logo":                   1,
				"figure":                 p.figure,
				"role_db_id":             "100001",
				"cards":                  cardIDs,
				"card_attrs":             cardAttrs,
				"passive_skills":         []any{},
				"defence_level":          max(int64(1), p.level),
				"defence_name":           "道馆挑战者",
				"defence_logo":           1,
				"defence_figure":         1,
				"defence_role_db_id":     "bot_1",
				"defence_cards":          []any{defID},
				"defence_card_attrs":     map[string]any{defID: map[string]any{"card_id": int64(11), "skin_id": int64(0), "fighting_point": int64(900), "level": max(int64(1), p.level), "advance": int64(1), "star": int64(2), "skills": map[int64]any{}, "attrs": defAttrs}},
				"defence_passive_skills": []any{},
			},
		},
		"view": map[string]any{
			"battleID":   battleID,
			"rand_seed":  time.Now().Unix() & 0x7fffffff,
			"myRank":     p.arenaRank,
			"battleRank": battleRank,
		},
	}
}

func (p *Service) pwBattleEnd(req any) map[string]any {
	inp := getInputMap(req)
	result, _ := inp["result"].(string)
	targetRank := toInt64(inp["rank"], p.arenaRank)
	now := time.Now().Unix()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.pvpPwTimes++
	p.pvpPwLastTime = now
	if result == "" || result == "win" {
		if targetRank > 0 && targetRank < p.arenaRank {
			p.arenaRank = targetRank
			if p.arenaRank < p.arenaRankTop {
				p.arenaRankTop = p.arenaRank
			}
		}
		p.arenaResultPoint += 2
		p.gold += 2000
	} else {
		p.arenaResultPoint += 1
		p.gold += 1000
	}
	p.syncVersion++

	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"result": result,
			"rank":   p.arenaRank,
			"award":  map[string]any{"gold": 2000},
		},
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{"gold": p.gold},
				},
				"arena": p.arenaModelSync(),
				"daily_record": map[string]any{
					"_db": map[string]any{
						"pvp_pw_times":     p.pvpPwTimes,
						"pvp_pw_last_time": p.pvpPwLastTime,
					},
				},
			},
		},
	}
}

func (p *Service) pwBattlePass(req any) map[string]any {
	inp := getInputMap(req)
	_ = toInt64(inp["battleRank"], p.arenaRank)
	now := time.Now().Unix()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.pvpPwTimes++
	p.pvpPwLastTime = now
	p.arenaResultPoint += 2
	p.gold += 1500
	p.syncVersion++

	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"award": map[string]any{"gold": 1500},
		},
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{"gold": p.gold},
				},
				"arena": p.arenaModelSync(),
				"daily_record": map[string]any{
					"_db": map[string]any{
						"pvp_pw_times":     p.pvpPwTimes,
						"pvp_pw_last_time": p.pvpPwLastTime,
					},
				},
			},
		},
	}
}

func (p *Service) pwBattleBuy(_ any) map[string]any {
	const costRMB int64 = 20
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.rmb < costRMB {
		return gameError("rmb_not_enough")
	}
	p.rmb -= costRMB
	p.buyPwTimes++
	p.syncVersion++

	return map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{"rmb": p.rmb},
				},
				"daily_record": map[string]any{
					"_db": map[string]any{"buy_pw_times": p.buyPwTimes},
				},
			},
		},
	}
}

func (p *Service) pwBattleCDBuy(_ any) map[string]any {
	const costRMB int64 = 10
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.rmb < costRMB {
		return gameError("rmb_not_enough")
	}
	p.rmb -= costRMB
	p.buyPwCdTimes++
	p.pvpPwLastTime = 0
	p.syncVersion++

	return map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{"rmb": p.rmb},
				},
				"daily_record": map[string]any{
					"_db": map[string]any{
						"buy_pw_cd_times":  p.buyPwCdTimes,
						"pvp_pw_last_time": p.pvpPwLastTime,
					},
				},
			},
		},
	}
}

func (p *Service) pwBattleItemUse(_ any) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.itemPwTimes++
	p.syncVersion++
	return map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"daily_record": map[string]any{
					"_db": map[string]any{
						"item_pw_times": p.itemPwTimes,
					},
				},
			},
		},
	}
}

func (p *Service) pwDisplay(req any) map[string]any {
	inp := getInputMap(req)
	cardID := toInt64(inp["card_id"], 21)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.arenaDisplay = cardID
	p.syncVersion++
	return map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"arena": map[string]any{
					"record": p.arenaRecordSnapshot(),
				},
			},
		},
	}
}

func (p *Service) pwRank(_ any) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()

	list := p.arenaTop10()
	list = append(list, p.arenaRecordSnapshot())
	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"rank": list,
		},
	}
}

func (p *Service) pwRoleInfo(req any) map[string]any {
	inp := getInputMap(req)
	recID := toInt64(inp["recordID"], 90001)
	rank := max(1, recID-90000)

	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"role": arenaBotEntry(rank, 21, 1500, fmt.Sprintf("训练师_%d", rank)),
		},
	}
}

func (p *Service) pwRankAward(req any) map[string]any {
	inp := getInputMap(req)
	csvID := toInt64(inp["csvID"], 0)
	if csvID <= 0 {
		return gameError("invalid_csv_id")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.pwRankAwards[csvID] = 0
	p.gold += 3000
	p.rmb += 30
	p.syncVersion++

	awards := make(map[int64]any, len(p.pwRankAwards))
	for k, v := range p.pwRankAwards {
		awards[k] = v
	}
	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"award": map[string]any{"gold": 3000, "rmb": 30},
		},
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"gold":          p.gold,
						"rmb":           p.rmb,
						"pw_rank_award": awards,
					},
				},
			},
		},
	}
}

func (p *Service) pwPointAward(req any) map[string]any {
	inp := getInputMap(req)
	csvID := toInt64(inp["csvID"], 0)
	if csvID <= 0 {
		return gameError("invalid_csv_id")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.pwPointAwards[csvID] = 0
	p.gold += 2000
	p.syncVersion++

	awards := make(map[int64]any, len(p.pwPointAwards))
	for k, v := range p.pwPointAwards {
		awards[k] = v
	}
	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"award": map[string]any{"gold": 2000},
		},
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{"gold": p.gold},
				},
				"daily_record": map[string]any{
					"_db": map[string]any{"result_point_award": awards},
				},
			},
		},
	}
}

func (p *Service) pwShopBuy(req any) map[string]any {
	inp := getInputMap(req)
	itemID := toInt64(inp["csvID"], 501)
	count := toInt64(inp["count"], 1)
	if itemID <= 0 || count <= 0 {
		return gameError("invalid_item")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.addItem(itemID, count)
	p.syncVersion++
	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"award": map[int64]any{itemID: count},
		},
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"items": maps.Clone(p.items),
					},
				},
			},
		},
	}
}
