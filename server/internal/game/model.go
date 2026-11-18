package game

import (
	"maps"
	"strconv"
	"time"
)

func (p *Service) login() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	accountID := p.accountID
	if accountID <= 0 {
		accountID = 100001
	}
	accountName := p.accountName
	if accountName == "" {
		accountName = "shuma_100001"
	}
	accountIDStr := strconv.FormatInt(accountID, 10)
	name := p.name
	figure := p.figure
	guides := make([]any, len(p.newbieGuide))
	for i, g := range p.newbieGuide {
		guides[i] = g
	}
	cardsCopy := make(map[string]any, len(p.cards))
	for k, v := range p.cards {
		cardsCopy[k] = snapshotModel(v)
	}
	cardIDs := p.cardIDs()
	gold := p.gold
	rmb := p.rmb
	level, levelExp := p.level, p.levelExp
	stamina, staminaAt := p.stamina, p.staminaAt
	items := maps.Clone(p.items)
	frags := maps.Clone(p.frags)
	house := p.houseSnapshot()
	p.sign.rollMonth(time.Now().In(signZone))
	signCount, signGift, signMonth := p.sign.total, p.sign.totalGift(), p.sign.monthlyModel()
	battleCards := make([]any, len(p.battleCards))
	for i, id := range p.battleCards {
		battleCards[i] = id
	}
	gates := p.gateSnapshot()
	deployment, readyCards := p.cardDeploymentSnapshot(), p.readyCardsSnapshot()
	pokedex := p.pokedexSnapshot()
	pokedexAdvance := maps.Clone(p.pokedexAdv)
	power, _, _ := p.rankMetrics()
	now := time.Now().Unix()
	p.drawDaily(time.Unix(now, 0))
	p.rollTaskDay(time.Unix(now, 0))
	return map[string]any{
		"ret":             true,
		"server_time":     now,
		"server_openTime": 1700000000,
		"global_record": map[string]any{
			"last_time":              now,
			"world_level":            1,
			"unionqa_round":          "closed",
			"normal_brave_challenge": map[string]any{},
		},
		"model": map[string]any{
			"account": map[string]any{
				"_db": map[string]any{
					"id":         accountID,
					"name":       accountName,
					"role_infos": map[string]any{},
				},
			},
			"role": map[string]any{
				"_db": map[string]any{
					"id":                        accountIDStr,
					"account_id":                accountIDStr,
					"name":                      name,
					"level":                     level,
					"level_exp":                 levelExp,
					"vip_level":                 p.vipLevel,
					"vip_sum":                   p.vipSum,
					"vip_hide":                  p.vipHide,
					"vip_gift":                  p.vipGiftSnapshot(),
					"recharges":                 map[string]any{},
					"ready_cards":               readyCards,
					"card_deployment":           deployment,
					"deployments_sync":          p.deploymentsSyncSnapshot(),
					"gold":                      gold,
					"rmb":                       rmb,
					"stamina":                   stamina,
					"stamina_last_recover_time": staminaAt,
					"stamina_free_times":        0,
					"trainer_level":             1,
					"trainer_level_exp":         0,
					"trainer_sum_exp":           0,
					"trainer_skills":            map[int64]any{},
					"trainer_attr_skills":       map[int64]any{},
					"figure":                    figure,
					"logo":                      1,
					"frame":                     1,
					"title_id":                  0,
					"titles":                    map[string]any{},
					"created_time":              1700000000,
					"logos":                     map[string]any{},
					"pokedex":                   pokedex,
					"figures":                   map[string]any{"1": 0, "2": 0},
					"skins":                     map[string]any{},
					"yyhuodongs":                map[string]any{},
					"cards":                     cardIDs,
					"battle_cards":              battleCards,
					"gate_star":                 gates["gate_star"],
					"gate_star_sum":             gates["gate_star_sum"],
					"gate_open":                 gates["gate_open"],
					"map_open":                      gates["map_open"],
					"map_star":                  gates["map_star"],
					"feres":                     map[string]any{},
					"house_lovers":              house,
					"mailbox":                       p.mailboxSnapshot(),
					"read_mailbox":                  p.readMailboxSnapshot(),
					"card_capacity":                 500,
					"dispatch_tasks":            []any{},
					"gym_datas": map[string]any{
						"gym_pass_awards":  map[string]any{},
						"gym_talent_point": 0,
						"gym_talent_trees": map[string]any{},
					},
					"items":                 items,
					"yycoins":               map[string]any{},
					"title_counter":         map[string]any{},
					"rmb_consume":           0,
					"sign_in_count":         signCount,
					"battle_fighting_point": 0,
					"top6_fighting_point":   power,
					"card_advance_times":    0,
					"newbie_guide":          guides,
					"city_sprites": map[string]any{
						"groups": []any{},
					},
					"online_gift":             map[string]any{},
					"grow_guide":              map[string]any{},
					"achievement_tasks":       map[string]any{},
					"achievement_box_awards":  []any{},
					"normal_brave_challenge":  map[string]any{},
					"passport":                map[string]any{"level": 0},
					"pokedex_advance":         pokedexAdvance,
					"skill_point":                   10,
					"skill_point_last_recover_time": now,
					"star_skill_points":             map[string]any{},
					"draw_sum_box":                  map[string]any{},
					"card_feels":              map[string]any{},
					"frags":                   frags,
					"zfrags":                  map[string]any{},
					"talent_trees":            map[string]any{},
					"talent_point":            0,
					"sign_in_gift":            signGift,
					"explorers":               map[string]any{},
					"held_items":              map[string]any{},
					"shop_limit":              map[string]any{},
					"pw_shop":                 map[int64]any{},
					"union_shop":              map[int64]any{},
					"explorer_shop":           map[int64]any{},
					"random_tower_shop":       map[int64]any{},
					"craft_shop":              map[int64]any{},
					"cross_craft_shop":        map[int64]any{},
					"fishing_shop":            map[int64]any{},
					"coin5":                   p.fragmentCoins,
					"huodongs":                map[string]any{},
					"huodong_cards":           map[int64]any{},
					"huodongs_gate":           map[int64]any{},
					"huodongs_index":          map[int64]any{},
					"daily_assistant":         map[string]any{},
					"endless_tower_current":   0,
					"endless_tower_max_gate":  0,
					"grid_walk":               map[string]any{},
					"reunion":                 map[string]any{},
					"pw_rank_award":           map[string]any{},
					"cross_online_fight_info": map[string]any{},
				},
				"_mem": map[string]any{
					"yy_delta": map[string]any{},
					"yy_open":  []any{},
				},
			},
			"cards": cardsCopy,
			"fishing": map[string]any{
				"_db": map[string]any{"select_scene": 0, "is_auto": false},
			},
			"random_tower": map[string]any{
				"_db": map[string]any{"point_award": map[string]any{}},
			},
			"daily_record": map[string]any{
				"_db": map[string]any{
					"city_sprite_gift_times":    0,
					"buy_stamina_times":         0,
					"buy_gold_times":            0,
					"union_mail_send_count":     0,
					"online_gift":               map[string]any{},
					"liveness_stage_award":      maps.Clone(p.livenessAwards),
					"liveness_point":            p.liveness,
					"gate_chanllenge":           p.gateDaily,
					"friend_stamina_gain":       0,
					"friend_stamina_send":       []any{},
					"item_dc1_free_counter":     0,
					"trainer_gift_times":        p.trainerGiftTimes,
					"dc1_free_count":            p.draws.freeDiamond,
					"splus_dc1_free_count":      0,
					"gold1_free_count":          p.draws.freeGold,
					"draw_card":                 p.draws.dailyTotal,
					"draw_card_rmb1_half":       p.draws.halfDiamond,
					"draw_card_gold1_trainer":   0,
					"gold1_free_last_time":      p.draws.lastFreeGold,
					"eq_dc1_free_counter":       0,
					"limit_box_free_counter":    0,
					"gem_gold_dc1_free_count":   0,
					"gem_rmb_dc1_free_count":    0,
					"limit_up_gem_free_count":   0,
					"boss_gate":                 0,
					"boss_gate_buy":             0,
					"lianjin_times":             p.lianjinTimes,
					"lianjin_free_times":        0,
					"lianjin_gifts":             p.lianjinGiftsSnapshot(),
					"gate_times":                map[string]any{},
					"buy_herogate_times":        map[string]any{},
					"union_training_speedup":    0,
					"huodong_boss_times":        0,
					"endless_tower_reset_times": 0,
					"craft_sign_up":             false,
					"fishing_counter":           0,
					"result_point_award":        map[string]any{},
					"pvp_enermys_refresh_times": p.pvpRefreshTimes,
					"buy_pw_times":              p.buyPwTimes,
					"pvp_pw_times":              p.pvpPwTimes,
					"pvp_pw_last_time":          p.pvpPwLastTime,
					"buy_pw_cd_times":           p.buyPwCdTimes,
					"item_pw_times":             p.itemPwTimes,
				},
			},
			"global_record": map[string]any{
				"_db": map[string]any{
					"huodong_gift_group":       1,
					"huodong_frag_group":       1,
					"huodong_gift_immune_type": 1,
				},
			},
			"monthly_record": map[string]any{"_db": signMonth},
			"lottery_record": map[string]any{
				"_db": map[string]any{
					"draw_card_up_choose":       p.drawUpChoose,
					"draw_card_up_change_times": 0,
					"draw_card_up1_counters":    map[string]any{},
					"draw_card_up10_counters":   map[string]any{},
					"dc1_counter":               0,
					"dc10_counter":              0,
					"splus_dc1_counter":         0,
					"splus_dc10_counter":        0,
					"eq_dc1_counter":            0,
					"yyhuodong_counters":        map[string]any{},
				},
			},
			"fix_shop":     p.shopModel("fix_shop"),
			"mystery_shop": p.shopModel("mystery_shop"),
			"frag_shop":    p.shopModel("frag_shop"),
			"union": map[string]any{
				"_db": map[string]any{
					"level": 0,
				},
			},
			"tasks": map[string]any{
				"_mem": p.tasksSnapshot(),
			},
			"society": map[string]any{
				"_db": map[string]any{
					"friends":      []any{},
					"stamina_recv": []any{},
					"friend_reqs":  []any{},
				},
			},
			"capture": map[string]any{
				"_db": map[string]any{
					"gate_sprites":  map[string]any{},
					"limit_sprites": map[string]any{},
				},
			},
			"arena": p.arenaModelSync(),
			"messages": map[string]any{
				"_mem": map[string]any{
					"news":  p.chatSlice("news"),
					"world": p.chatSlice("world"),
					"union": p.chatSlice("union"),
					"cross": p.chatSlice("cross"),
				},
			},
		},
	}
}
