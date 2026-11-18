package game

import (
	"fmt"
	"maps"
	"time"
)

func (p *Service) pokedexAdvance(req any) map[string]any {
	inp := getInputMap(req)
	csvID := toInt64(inp["csvID"], 0)
	if csvID <= 0 {
		return gameError("invalid_csv_id")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.pokedexAdv[csvID] = 1
	p.syncVersion++
	return p.roleSync(map[string]any{
		"pokedex_advance": maps.Clone(p.pokedexAdv),
	})
}

func (p *Service) talentLevelUpReady(req any) map[string]any {
	inp := getInputMap(req)
	talentID := toInt64(inp["talentID"], 0)
	if talentID <= 0 {
		return gameError("invalid_talent")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.talents[talentID] = toInt64(p.talents[talentID], 0) + 1
	p.syncVersion++
	return p.roleSync(map[string]any{
		"talent_trees": map[int64]any{1: maps.Clone(p.talents)},
		"talent_point": p.talentPoint,
	})
}

func (p *Service) talentLevelUpEnd(req any) map[string]any {
	inp := getInputMap(req)
	p.mu.Lock()
	defer p.mu.Unlock()

	if list, ok := inp["talentIDs"].([]any); ok {
		for _, item := range list {
			id := toInt64(item, 0)
			if id > 0 {
				p.talents[id] = toInt64(p.talents[id], 0) + 1
			}
		}
	}
	p.syncVersion++
	return p.roleSync(map[string]any{
		"talent_trees": map[int64]any{1: maps.Clone(p.talents)},
		"talent_point": p.talentPoint,
	})
}

func (p *Service) talentReset(_ any) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.talents = map[int64]any{}
	p.syncVersion++
	return p.roleSync(map[string]any{
		"talent_trees": map[string]any{},
		"talent_point": p.talentPoint,
	})
}

func (p *Service) achievementTaskAward(req any) map[string]any {
	inp := getInputMap(req)
	csvID := toInt64(inp["csvID"], 0)
	if csvID <= 0 {
		return gameError("invalid_csv_id")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.achievementTasks[csvID] = 0
	p.gold += 1000
	p.rmb += 20
	p.syncVersion++
	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"award": map[string]any{"gold": 1000, "rmb": 20},
		},
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"gold":              p.gold,
						"rmb":               p.rmb,
						"achievement_tasks": maps.Clone(p.achievementTasks),
					},
				},
			},
		},
	}
}

func (p *Service) achievementBoxAward(req any) map[string]any {
	inp := getInputMap(req)
	csvID := toInt64(inp["csvID"], 0)
	if csvID <= 0 {
		return gameError("invalid_csv_id")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.achievementBoxes = append(p.achievementBoxes, csvID)
	p.rmb += 50
	p.syncVersion++
	boxes := make([]any, len(p.achievementBoxes))
	for i, id := range p.achievementBoxes {
		boxes[i] = id
	}
	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"award": map[string]any{"rmb": 50},
		},
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"rmb":                    p.rmb,
						"achievement_box_awards": boxes,
					},
				},
			},
		},
	}
}

func (p *Service) citySpriteGift(_ any) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.citySpriteGifts++
	p.gold += 500
	p.syncVersion++
	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"award": map[string]any{"gold": 500},
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
						"city_sprite_gift_times": p.citySpriteGifts,
					},
				},
			},
		},
	}
}

type mailItem struct {
	ID      string
	Type    int64
	Subject string
	Content string
	Sender  string
	Time    int64
	RMB     int64
	Gold    int64
}

func (m mailItem) wire() map[string]any {
	attachs := map[string]any{}
	if m.RMB > 0 {
		attachs["rmb"] = m.RMB
	}
	if m.Gold > 0 {
		attachs["gold"] = m.Gold
	}
	return map[string]any{
		"db_id":     m.ID,
		"type":      m.Type,
		"subject":   m.Subject,
		"content":   m.Content,
		"sender":    m.Sender,
		"time":      m.Time,
		"hasattach": len(attachs) > 0,
		"attachs":   attachs,
	}
}

func defaultMailbox() []mailItem {
	return []mailItem{
		{
			ID:      "mail_9999999_rmb",
			Type:    1,
			Subject: "测试福利：9999999钻石",
			Content: "欢迎测试！请查收为您发放的 9,999,999 钻石福利，祝您游戏愉快！",
			Sender:  "系统管理员",
			Time:    time.Now().Unix(),
			RMB:     9999999,
		},
	}
}

func mailSliceWire(list []mailItem) []any {
	out := make([]any, len(list))
	for i, m := range list {
		out[i] = m.wire()
	}
	return out
}

func (p *Service) mailboxSnapshot() []any {
	return mailSliceWire(p.mailbox)
}

func (p *Service) readMailboxSnapshot() []any {
	return mailSliceWire(p.readMailbox)
}

func (p *Service) mailGet(req any) map[string]any {
	inp := getInputMap(req)
	mailID, _ := inp["mailID"].(string)

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, m := range p.mailbox {
		if m.ID == mailID || mailID == "" {
			return map[string]any{"ret": true, "view": m.wire()}
		}
	}
	for _, m := range p.readMailbox {
		if m.ID == mailID {
			return map[string]any{"ret": true, "view": m.wire()}
		}
	}
	return gameError("mail_not_found")
}

func (p *Service) mailRead(req any) map[string]any {
	inp := getInputMap(req)
	mailID, _ := inp["mailID"].(string)

	p.mu.Lock()
	defer p.mu.Unlock()

	idx := -1
	for i, m := range p.mailbox {
		if m.ID == mailID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return gameError("mail_not_found")
	}
	m := p.mailbox[idx]
	p.mailbox = append(p.mailbox[:idx], p.mailbox[idx+1:]...)
	p.readMailbox = append(p.readMailbox, m)
	p.rmb += m.RMB
	p.gold += m.Gold
	p.syncVersion++

	view := map[string]any{}
	if m.RMB > 0 {
		view["rmb"] = m.RMB
	}
	if m.Gold > 0 {
		view["gold"] = m.Gold
	}
	reply := p.roleSync(map[string]any{
		"rmb":          p.rmb,
		"gold":         p.gold,
		"mailbox":      p.mailboxSnapshot(),
		"read_mailbox": p.readMailboxSnapshot(),
	})
	reply["view"] = view
	reply["sync"].(map[string]any)["new"] = map[string]any{
		"role": map[string]any{
			"_db": map[string]any{
				"mailbox":      true,
				"read_mailbox": true,
			},
		},
	}
	return reply
}

func (p *Service) mailReadAll(_ any) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()

	var totalRMB, totalGold int64
	for _, m := range p.mailbox {
		totalRMB += m.RMB
		totalGold += m.Gold
		p.readMailbox = append(p.readMailbox, m)
	}
	p.mailbox = nil
	p.rmb += totalRMB
	p.gold += totalGold
	p.syncVersion++

	view := map[string]any{}
	if totalRMB > 0 {
		view["rmb"] = totalRMB
	}
	if totalGold > 0 {
		view["gold"] = totalGold
	}
	reply := p.roleSync(map[string]any{
		"rmb":          p.rmb,
		"gold":         p.gold,
		"mailbox":      p.mailboxSnapshot(),
		"read_mailbox": p.readMailboxSnapshot(),
	})
	reply["view"] = view
	reply["sync"].(map[string]any)["new"] = map[string]any{
		"role": map[string]any{
			"_db": map[string]any{
				"mailbox":      true,
				"read_mailbox": true,
			},
		},
	}
	return reply
}

func (p *Service) mailDelete(_ any) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.readMailbox = nil
	p.syncVersion++
	reply := p.roleSync(map[string]any{
		"read_mailbox": p.readMailboxSnapshot(),
	})
	reply["sync"].(map[string]any)["new"] = map[string]any{
		"role": map[string]any{
			"_db": map[string]any{
				"read_mailbox": true,
			},
		},
	}
	return reply
}

type chatMsg struct {
	ID     string
	Msg    string
	Time   int64
	Type   string
	RoleID string
	Name   string
	Level  int64
	VIP    int64
	Logo   int64
	Frame  int64
}

func (c chatMsg) wire() map[string]any {
	return map[string]any{
		"id":   c.ID,
		"msg":  c.Msg,
		"time": c.Time,
		"type": c.Type,
		"role": map[string]any{
			"id":     c.RoleID,
			"name":   c.Name,
			"level":  c.Level,
			"vip":    c.VIP,
			"logo":   c.Logo,
			"frame":  c.Frame,
			"title":  int64(0),
			"server": "game.cn.1",
		},
	}
}

func defaultChatMessages() map[string][]chatMsg {
	now := time.Now().Unix()
	return map[string][]chatMsg{
		"news": {
			{
				ID: "chat_1", Msg: "欢迎来到本地测试服，祝您游戏愉快！", Time: now, Type: "news",
				RoleID: "100000", Name: "系统公告", Level: 99, VIP: 0, Logo: 1, Frame: 1,
			},
		},
		"world": {
			{
				ID: "chat_world_1", Msg: "欢迎来到本地测试服，世界频道已开启！", Time: now, Type: "world",
				RoleID: "100000", Name: "系统公告", Level: 99, VIP: 0, Logo: 1, Frame: 1,
			},
		},
		"union": {},
		"cross": {},
	}
}

func (p *Service) chatSlice(channel string) []any {
	list := p.chatMessages[channel]
	out := make([]any, len(list))
	for i, m := range list {
		out[i] = m.wire()
	}
	return out
}

func (p *Service) sendChat(req any) map[string]any {
	inp := getInputMap(req)
	msg, _ := inp["msg"].(string)
	if msg == "" {
		return gameError("msg_empty")
	}
	chType, _ := inp["type"].(string)
	if chType == "" {
		chType = "world"
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.chatMessages == nil {
		p.chatMessages = defaultChatMessages()
	}
	p.nextChatID++
	item := chatMsg{
		ID:     fmt.Sprintf("chat_%d", p.nextChatID),
		Msg:    msg,
		Time:   time.Now().Unix(),
		Type:   chType,
		RoleID: fmt.Sprintf("%d", p.accountID),
		Name:   p.name,
		Level:  p.level,
		VIP:    p.vipLevel,
		Logo:   1,
		Frame:  1,
	}
	p.chatMessages[chType] = append(p.chatMessages[chType], item)
	p.syncVersion++

	return map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": p.syncVersion,
			"new": map[string]any{
				"messages": map[string]any{
					"_mem": map[string]any{
						chType: true,
					},
				},
			},
			"upd": map[string]any{
				"messages": map[string]any{
					"_mem": map[string]any{
						chType: p.chatSlice(chType),
					},
				},
			},
		},
	}
}
