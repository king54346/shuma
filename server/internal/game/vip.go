package game

import "maps"

func (p *Service) vipGiftSnapshot() map[int64]any {
	out := make(map[int64]any, len(p.vipGift))
	for k, v := range p.vipGift {
		out[k] = v
	}
	return out
}

func (p *Service) buyVipGift(req any) map[string]any {
	inp := getInputMap(req)
	vipLv := toInt64(inp["vipLevel"], -1)
	if vipLv < 0 || vipLv > 18 {
		return gameError("invalid_vip_level")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if vipLv > p.vipLevel {
		return gameError("vip_level_not_enough")
	}
	if bought, ok := p.vipGift[vipLv]; ok && bought == 0 {
		return gameError("vip_gift_already_bought")
	}

	costRMB := vipLv * 10
	if p.rmb < costRMB {
		return gameError("rmb_not_enough")
	}
	p.rmb -= costRMB
	p.vipGift[vipLv] = 0 // 0 means bought in RechargeView
	gainGold := 5000 * (vipLv + 1)
	p.gold += gainGold
	p.addItem(501, 5)
	p.syncVersion++

	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"award": map[string]any{
				"gold": gainGold,
				"501":  5,
			},
		},
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"rmb":      p.rmb,
						"gold":     p.gold,
						"items":    maps.Clone(p.items),
						"vip_gift": p.vipGiftSnapshot(),
					},
				},
			},
		},
	}
}

func (p *Service) switchVipDisplay(req any) map[string]any {
	inp := getInputMap(req)
	flag, ok := inp["flag"].(bool)
	if !ok {
		flag = !p.vipHide
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.vipHide = flag
	p.syncVersion++
	return p.roleSync(map[string]any{
		"vip_hide": p.vipHide,
	})
}

func (p *Service) claimVipMonthGift(_ any) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.vipMonthGiftClaimed {
		return gameError("vip_month_gift_claimed")
	}
	p.vipMonthGiftClaimed = true
	p.gold += 10000
	p.addItem(502, 2)
	p.syncVersion++

	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"award": map[string]any{
				"gold": 10000,
				"502":  2,
			},
		},
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd": map[string]any{
				"role": map[string]any{
					"_db": map[string]any{
						"gold":  p.gold,
						"items": maps.Clone(p.items),
					},
				},
				"monthly_record": map[string]any{
					"_db": map[string]any{
						"vip_gift": map[int64]any{1: 0},
					},
				},
			},
		},
	}
}

func (p *Service) claimVipWeekGift(req any) map[string]any {
	inp := getInputMap(req)
	giftType := toInt64(inp["type"], 1)

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.vipWeekGifts[giftType] == 0 {
		if _, exists := p.vipWeekGifts[giftType]; exists {
			return gameError("vip_week_gift_claimed")
		}
	}
	p.vipWeekGifts[giftType] = 0
	p.gold += 5000
	p.syncVersion++

	return map[string]any{
		"ret": true,
		"view": map[string]any{
			"award": map[string]any{
				"gold": 5000,
			},
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
