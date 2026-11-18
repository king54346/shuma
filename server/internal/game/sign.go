package game

import (
	"maps"
	"time"
)

var signZone = time.FixedZone("UTC+8", 8*60*60)

var signMonthMilestones = map[int64]int64{101: 7, 102: 15, 103: 28}
var signBuyCost = [...]int64{0, 0, 0, 10, 20, 30, 50, 50, 50, 100, 100, 100, 200}

type signState struct {
	month, count, lastDay, lastIndex, buys int64
	total, claimed                         int64
	monthGifts                             map[int64]any
	awards                                 map[int64]any
}

func (s *signState) rollMonth(now time.Time) {
	month := int64(now.Year()*100 + int(now.Month()))
	if s.month != month {
		*s = signState{
			month: month, total: s.total, claimed: s.claimed,
			monthGifts: make(map[int64]any), awards: make(map[int64]any),
		}
	}
}

func (s *signState) totalGift() []any {
	next := s.claimed + 1
	if next >= int64(len(signTotalRewards)) {
		return []any{int64(len(signTotalRewards) - 1), 0}
	}
	state := 0
	if s.total >= signTotalRewards[next].days {
		state = 1
	}
	return []any{next, state}
}

func (s *signState) monthlyModel() map[string]any {
	return map[string]any{
		"month": s.month, "sign_in": s.count,
		"last_sign_in_day": s.lastDay, "last_sign_in_idx": s.lastIndex,
		"sign_in_buy_times": s.buys, "vip": 0, "vip_gift": map[string]any{},
		"sign_in_gift": maps.Clone(s.monthGifts), "sign_in_awards": maps.Clone(s.awards),
	}
}

func (p *Service) signIn(route string, req any) map[string]any {
	input := getInputMap(req)
	today := time.Now().In(signZone)
	p.mu.Lock()
	defer p.mu.Unlock()
	s := &p.sign
	s.rollMonth(today)

	var reward gateReward
	switch route {
	case "/game/role/sign_in", "/game/role/sign_in/buy":
		buy := route == "/game/role/sign_in/buy"
		day := int64(today.Day())
		if !buy && s.lastDay == day {
			fields := make(map[string]any)
			view := p.applyReward(signReward(s.month, s.lastIndex), true, fields)
			return p.signReply(fields, view)
		}
		if buy {
			csvID := toInt64(input["csvID"], 0)
			if csvID > 0 {
				// VIP double-reward claim for a signed day.
				s.awards[csvID] = map[int64]any{csvID: 0}
				reward = signReward(s.month, csvID)
				break
			}
			if s.count >= day {
				return gameError("sign_in_buy_unavailable")
			}
			cost := signBuyCost[min(s.buys, int64(len(signBuyCost)-1))]
			if p.rmb < cost {
				return gameError("insufficient_rmb")
			}
			p.rmb -= cost
			s.buys++
		} else {
			s.lastDay = day
		}
		s.count++
		s.total++
		if !buy {
			s.lastIndex = s.count
			s.awards[day] = map[int64]any{s.count: 0}
		}
		for id, days := range signMonthMilestones {
			if s.count >= days {
				if _, reached := s.monthGifts[id]; !reached {
					s.monthGifts[id] = 1
				}
			}
		}
		reward = signReward(s.month, s.count)
	case "/game/role/sign_in/month/total_award":
		id := toInt64(input["csvID"], 0)
		if toInt64(s.monthGifts[id], 0) != 1 {
			return gameError("sign_in_reward_unavailable")
		}
		s.monthGifts[id] = 0
		reward = signReward(s.month, id)
	case "/game/role/sign_in/total_award":
		next := s.claimed + 1
		if next >= int64(len(signTotalRewards)) || s.total < signTotalRewards[next].days {
			return gameError("sign_in_reward_unavailable")
		}
		s.claimed = next
		reward = signTotalRewards[next].reward
	}

	fields := make(map[string]any)
	view := p.applyReward(reward, false, fields)
	return p.signReply(fields, view)
}

func signReward(month, id int64) gateReward {
	if reward, ok := signMonthOverrides[[2]int64{id, month % 100}]; ok {
		return reward
	}
	return signRewards[id]
}

func (p *Service) signReply(fields, view map[string]any) map[string]any {
	fields["sign_in_count"], fields["sign_in_gift"] = p.sign.total, p.sign.totalGift()
	reply := p.roleSync(fields)
	syncData := reply["sync"].(map[string]any)
	syncData["upd"].(map[string]any)["monthly_record"] = map[string]any{"_db": p.sign.monthlyModel()}
	syncData["new"] = map[string]any{
		"role":           map[string]any{"_db": map[string]any{"sign_in_gift": true}},
		"monthly_record": map[string]any{"_db": true},
	}
	reply["view"] = view
	return reply
}
