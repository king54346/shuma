package game

const loverMaxFeel int64 = 300

// Unlock gates and gift values come from banlvxiaowu and items in the local APK.
var loverUnlockGates = [...]int64{10102, 10205, 10407, 10609, 10809, 11009, 11209, 11509, 11709, 11909, 12109}
var loverGiftFeel = map[int64]int64{9000: 1, 9001: 5, 9002: 10, 9003: 20, 9004: 100}

type lover struct {
	feel int64
	card string
}

func (l lover) wire() map[string]any {
	return map[string]any{"feel": l.feel, "card": l.card}
}

func (p *Service) houseSnapshot() map[int64]any {
	out := make(map[int64]any, len(p.houseLovers))
	for id, l := range p.houseLovers {
		out[id] = l.wire()
	}
	return out
}

func (p *Service) loverUnlocked(id int64) bool {
	if id < 1 || id > int64(len(loverUnlockGates)) {
		return false
	}
	if _, exists := p.houseLovers[id]; exists {
		return true
	}
	return p.gateStar[loverUnlockGates[id-1]].stars > 0
}

func (p *Service) giftLover(req any) map[string]any {
	input := getInputMap(req)
	id := toInt64(input["loveID"], 0)
	item := toInt64(input["itemID"], 0)
	count := toInt64(input["itemCount"], 0)
	feel := loverGiftFeel[item]
	if feel == 0 || count <= 0 || count > loverMaxFeel {
		return gameError("invalid_lover_gift")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.loverUnlocked(id) {
		return gameError("lover_locked")
	}
	l := p.houseLovers[id]
	if l.feel >= loverMaxFeel {
		return gameError("lover_feel_full")
	}
	total := toInt64(p.items[item], 0)
	if total < count {
		return gameError("insufficient_items")
	}

	l.feel = min(loverMaxFeel, l.feel+feel*count)
	p.houseLovers[id] = l
	p.items[item] = total - count
	p.syncVersion++
	return p.roleSync(map[string]any{
		"house_lovers": map[int64]any{id: l.wire()},
		"items":        map[int64]any{item: p.items[item]},
	})
}

func (p *Service) blessCard(req any) map[string]any {
	input := getInputMap(req)
	id := toInt64(input["loveID"], 0)
	card, _ := input["cardID"].(string)
	p.mu.Lock()
	defer p.mu.Unlock()

	l := p.houseLovers[id]
	if !p.loverUnlocked(id) || l.feel < loverMaxFeel {
		return gameError("lover_bless_locked")
	}
	if _, exists := p.cards[card]; !exists {
		return gameError("card_not_found")
	}
	for otherID, other := range p.houseLovers {
		if otherID != id && other.card == card {
			return gameError("card_already_blessed")
		}
	}
	if l.card != card {
		l.card = card
		p.houseLovers[id] = l
		p.syncVersion++
	}
	return p.roleSync(map[string]any{"house_lovers": map[int64]any{id: l.wire()}})
}
