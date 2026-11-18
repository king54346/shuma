package game

import (
	"maps"
	"time"
)

type shopOffer struct {
	slot, csvID, itemID, quantity int64
	currency                      string
	price                         int64
}

type shopState struct {
	Cycle   int64
	Updated int64
	Bought  map[int64]int64
}

// Local selections are taken from the APK shop tables at their original positions.
var menuShops = map[string][][]shopOffer{
	"fix_shop": {
		{{1, 1021, 21351, 1, "gold", 10000}, {2, 12, 5000, 10, "gold", 60000}, {3, 32, 5000, 10, "rmb", 60}, {5, 61, 524, 1, "rmb", 60}},
		{{1, 1023, 21351, 1, "rmb", 10}, {2, 11, 5000, 20, "gold", 120000}, {3, 31, 5000, 20, "rmb", 120}, {5, 61, 524, 1, "rmb", 60}},
	},
	"mystery_shop": {
		{{1, 12, 20001, 1, "gold", 60000}, {3, 31, 20001, 1, "rmb", 120}, {4, 41, 20001, 1, "gold", 20000}},
		{{1, 11, 20001, 1, "gold", 120000}, {3, 1201, 20881, 1, "rmb", 50}, {4, 41, 20001, 1, "gold", 20000}},
	},
	"frag_shop": {
		{{1, 101, 22031, 5, "coin5", 200}, {2, 111, 20881, 5, "coin5", 200}, {7, 203, 21351, 5, "coin5", 80}},
	},
}

func (p *Service) shopModel(key string) map[string]any {
	state := p.shops[key]
	variant := menuShops[key]
	items := map[int64]any{}
	for _, offer := range variant[state.Cycle%int64(len(variant))] {
		items[offer.slot] = []any{offer.csvID, offer.itemID}
	}
	return map[string]any{"_db": map[string]any{
		"items": items, "buy": wireCounts(state.Bought), "last_active_time": state.Updated,
	}}
}

func (p *Service) shopReply(key string, fields map[string]any) map[string]any {
	reply := p.roleSync(fields)
	reply["sync"].(map[string]any)["upd"].(map[string]any)[key] = p.shopModel(key)
	reply["sync"].(map[string]any)["new"] = map[string]any{
		key: map[string]any{"_db": map[string]any{"items": true, "buy": true}},
	}
	return reply
}

func (p *Service) shopGet(shopKey string) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.shopReply(shopKey, map[string]any{})
}

func (p *Service) shopRefresh(shopKey string) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(menuShops[shopKey]) < 2 {
		return gameError("shop_refresh_unavailable")
	}
	if p.rmb < 20 {
		return gameError("rmb_not_enough")
	}
	p.rmb -= 20
	state := p.shops[shopKey]
	state.Cycle++
	state.Updated = time.Now().Unix()
	state.Bought = nil
	p.shops[shopKey] = state
	p.syncVersion++
	return p.shopReply(shopKey, map[string]any{"rmb": p.rmb})
}

func (p *Service) shopBuy(req any, shopKey string) map[string]any {
	inp := getInputMap(req)
	slot := toInt64(inp["idx"], 0)
	csvID := toInt64(inp["shopID"], 0)
	itemID := toInt64(inp["itemID"], 0)
	count := toInt64(inp["count"], 1)
	if slot <= 0 || count < 1 || count > 99 {
		return gameError("invalid_shop_purchase")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	state := p.shops[shopKey]
	variant := menuShops[shopKey]
	for _, offer := range variant[state.Cycle%int64(len(variant))] {
		if offer.slot != slot || offer.csvID != csvID || offer.itemID != itemID {
			continue
		}
		price := offer.price * count
		switch offer.currency {
		case "gold":
			if p.gold < price {
				return gameError("gold_not_enough")
			}
			p.gold -= price
		case "rmb":
			if p.rmb < price {
				return gameError("rmb_not_enough")
			}
			p.rmb -= price
		case "coin5":
			if p.fragmentCoins < price {
				return gameError("shop_currency_not_enough")
			}
			p.fragmentCoins -= price
		default:
			return gameError("shop_currency_not_enough")
		}
		if state.Bought == nil {
			state.Bought = make(map[int64]int64)
		}
		state.Bought[slot] += count
		p.shops[shopKey] = state
		n := offer.quantity * count
		if itemID >= 20000 && itemID < 30000 {
			p.addFrag(itemID, n)
		} else {
			p.addItem(itemID, n)
		}
		p.syncVersion++
		reply := p.shopReply(shopKey, map[string]any{
			"gold": p.gold, "rmb": p.rmb, "coin5": p.fragmentCoins,
			"items": maps.Clone(p.items), "frags": maps.Clone(p.frags),
		})
		replaceRoleFields(reply, "items", "frags")
		reply["view"] = map[string]any{"items": []any{[]any{itemID, n}}}
		return reply
	}
	return gameError("shop_offer_changed")
}
func (p *Service) buyItem(req any) map[string]any {
	inp := getInputMap(req)
	itemID := toInt64(inp["itemID"], 0)
	count := toInt64(inp["itemCount"], 1)
	if itemID <= 0 || count <= 0 {
		return gameError("invalid_item")
	}
	costRMB := 10 * count

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.rmb < costRMB {
		return gameError("rmb_not_enough")
	}
	p.rmb -= costRMB
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
						"rmb":   p.rmb,
						"items": maps.Clone(p.items),
					},
				},
			},
		},
	}
}
