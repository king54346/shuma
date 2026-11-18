package game

func (p *Service) saveGuide(req any) map[string]any {
	inp := getInputMap(req)
	guideID := toInt64(inp["guideID"], -1)
	p.mu.Lock()
	guides := p.addGuideID(guideID)
	p.syncVersion++
	ver := p.syncVersion
	p.mu.Unlock()
	return map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": ver,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"newbie_guide": guides,
					},
				},
			},
		},
	}
}

func (p *Service) initRole(req any) map[string]any {
	inp := getInputMap(req)
	guideID := toInt64(inp["guideID"], 1)
	figure := toInt64(inp["figure"], 1)
	name, _ := inp["name"].(string)
	if name == "" {
		name = "本地测试训练师"
	}
	p.mu.Lock()
	p.name = name
	p.figure = figure
	guides := p.addGuideID(guideID)
	p.syncVersion++
	ver := p.syncVersion
	p.mu.Unlock()
	return map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": ver,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"name":         name,
						"figure":       figure,
						"newbie_guide": guides,
					},
				},
			},
		},
	}
}

func (p *Service) chooseStarter(req any) map[string]any {
	inp := getInputMap(req)
	guideID := toInt64(inp["guideID"], 2)
	cardID := toInt64(inp["cardID"], 1)
	cardDBID := "1001"
	cardObj := newCard(cardDBID, cardID, 1)
	p.mu.Lock()
	p.cards[cardDBID] = cardObj
	p.registerSpecies(cardID)
	p.battleCards = []string{cardDBID}
	allCardIDs := p.cardIDs()
	guides := p.addGuideID(guideID)
	p.syncVersion++
	ver := p.syncVersion
	p.mu.Unlock()
	return map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": ver,
			"new":     map[string]any{},
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"newbie_guide": guides,
						"cards":        allCardIDs,
						"battle_cards": []any{cardDBID},
					},
				},
				"cards": map[string]any{
					cardDBID: cardObj,
				},
			},
		},
	}
}

func (p *Service) claimGuideAward(req any) map[string]any {
	id := toInt64(getInputMap(req)["guideCsvID"], 0)
	p.mu.Lock()
	defer p.mu.Unlock()

	dbID, claimed := p.guideAwards[id]
	view := make(map[string]any)
	role := make(map[string]any)
	models := map[string]any{"role": map[string]any{"_db": role}}
	syncData := map[string]any{"upd": models}
	var stage int64
	switch id {
	case 103:
		stage = 10
		if !claimed {
			dbID = p.allocateCardID()
			p.cards[dbID] = newCard(dbID, 92, 5)
			p.registerSpecies(92)
		}
		role["cards"] = p.cardIDs()
		models["cards"] = map[string]any{dbID: p.cards[dbID]}
		view["carddbIDs"] = []any{[]any{dbID, true}}
	case 164:
		stage = 17
		p.applyGuideItem(9004, 3, claimed, view, role)
	case 342:
		stage = 35
		p.applyGuideItem(519, 10, claimed, view, role)
	case 500:
		stage = 50
		if !claimed {
			p.gold += 30000
		}
		role["gold"] = p.gold
		view["gold"] = 30000
	default:
		return map[string]any{"ret": false, "err": "unsupported_guide_award"}
	}
	role["newbie_guide"] = p.addGuideID(stage)
	if !claimed {
		p.guideAwards[id] = dbID
		p.syncVersion++
	}
	syncData["version"] = p.syncVersion
	return map[string]any{"ret": true, "view": view, "sync": syncData}
}

func (p *Service) applyGuideItem(id, count int64, claimed bool, view, role map[string]any) {
	total := toInt64(p.items[id], 0)
	if !claimed {
		total += count
		p.items[id] = total
	}
	view["items"] = []any{[]any{id, count}}
	role["items"] = map[int64]any{id: total}
}
