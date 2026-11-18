package game

import (
	"testing"
	"time"

	"shuma-server/internal/login"
	"shuma-server/internal/playerstore"
)

func TestBagAndFragRoutes(t *testing.T) {
	svc := New()
	svc.items[501] = int64(5)
	svc.frags[20011] = int64(30)

	// Use 2 items
	res := svc.Handle("/game/role/item/use", map[string]any{
		"input": map[string]any{"itemsD": map[string]any{"501": 2}},
	})
	if res["ret"] != true || svc.itemCount(501) != 3 || svc.gold != 11000 {
		t.Fatalf("useItems failed: res=%v count=%d gold=%d", res, svc.itemCount(501), svc.gold)
	}

	// Sell 1 item
	res = svc.Handle("/game/role/item/sell", map[string]any{
		"input": map[string]any{"itemsD": map[string]any{"501": 1}},
	})
	if res["ret"] != true || svc.itemCount(501) != 2 || svc.gold != 11100 {
		t.Fatalf("sellItems failed: res=%v count=%d gold=%d", res, svc.itemCount(501), svc.gold)
	}

	// Insufficient item fails cleanly
	errRes := svc.Handle("/game/role/item/use", map[string]any{
		"input": map[string]any{"itemsD": map[string]any{"501": 99}},
	})
	if errRes["ret"] != false || errRes["err"] != "item_not_enough" {
		t.Fatalf("expected item_not_enough, got %v", errRes)
	}

	// Combine 30 frags into a new card
	comb := svc.Handle("/game/role/frag/comb", map[string]any{
		"input": map[string]any{"fragID": 20011},
	})
	if comb["ret"] != true || svc.fragCount(20011) != 0 || len(svc.cards) != 2 {
		t.Fatalf("combineFrag failed: res=%v frags=%d cards=%d", comb, svc.fragCount(20011), len(svc.cards))
	}

	// Buy stamina & lianjin & rename
	stRes := svc.Handle("/game/role/stamina/buy", map[string]any{})
	if stRes["ret"] != true || svc.stamina != 240 || svc.rmb != 950 {
		t.Fatalf("buyStamina failed: res=%v stamina=%d rmb=%d", stRes, svc.stamina, svc.rmb)
	}
	ljRes := svc.Handle("/game/role/lianjin", map[string]any{"input": map[string]any{"count": 2}})
	if ljRes["ret"] != true || svc.gold != 51100 || svc.rmb != 930 {
		t.Fatalf("lianjin failed: res=%v gold=%d rmb=%d", ljRes, svc.gold, svc.rmb)
	}
	rnRes := svc.Handle("/game/role/rename", map[string]any{"input": map[string]any{"name": "新训练师"}})
	if rnRes["ret"] != true || svc.name != "新训练师" {
		t.Fatalf("rename failed: res=%v name=%q", rnRes, svc.name)
	}
}

func TestCardDevelopRoutes(t *testing.T) {
	svc := New()
	// Draw a second card ("1002") to use as star-up material
	drawRes := svc.Handle("/game/lottery/card/draw", map[string]any{"input": map[string]any{"drawType": "free1"}})
	if drawRes["ret"] != true || len(svc.cards) != 2 {
		t.Fatalf("draw failed: %v", drawRes)
	}

	// Three experience items raise the card level.
	svc.items[11] = int64(3)
	expRes := svc.Handle("/game/card/exp/use_item", map[string]any{
		"input": map[string]any{"cardID": "1001", "itemID": 11, "itemCount": 3},
	})
	if expRes["ret"] != true {
		t.Fatalf("cardExpUseItem failed: %v", expRes)
	}
	_, db, _ := svc.getCardDB("1001")
	if toInt64(db["level"], 1) <= 1 {
		t.Fatalf("expected level > 1 after 300 exp, got %v", db["level"])
	}

	// Advance + skill + equip
	if res := svc.Handle("/game/card/advance", map[string]any{"input": map[string]any{"cardID": "1001", "count": 1}}); res["ret"] != true {
		t.Fatalf("cardAdvance failed: %v", res)
	}
	if res := svc.Handle("/game/card/skill/level/up", map[string]any{"input": map[string]any{"cardID": "1001", "skillID": 101, "addLevel": 2}}); res["ret"] != true {
		t.Fatalf("cardSkillLevelUp failed: %v", res)
	}
	if res := svc.Handle("/game/equip/strength", map[string]any{"input": map[string]any{"cardID": "1001", "equipPos": 1, "upLevel": 3}}); res["ret"] != true {
		t.Fatalf("equipStrength failed: %v", res)
	}

	// Lock "1002" -> star up must reject locked card
	if res := svc.Handle("/game/card/locked/switch", map[string]any{"input": map[string]any{"cardID": "1002"}}); res["ret"] != true {
		t.Fatalf("cardLockSwitch failed: %v", res)
	}
	errStar := svc.Handle("/game/card/star", map[string]any{
		"input": map[string]any{"cardID": "1001", "costCardIDs": []any{"1002"}},
	})
	if errStar["ret"] != false || errStar["err"] != "card_locked" {
		t.Fatalf("expected card_locked, got %v", errStar)
	}

	// Unlock "1002" -> star up succeeds and consumes "1002"
	svc.Handle("/game/card/locked/switch", map[string]any{"input": map[string]any{"cardID": "1002"}})
	okStar := svc.Handle("/game/card/star", map[string]any{
		"input": map[string]any{"cardID": "1001", "costCardIDs": []any{"1002"}},
	})
	if okStar["ret"] != true || toInt64(db["star"], 1) != 2 || len(svc.cards) != 1 {
		t.Fatalf("cardStarUp failed: res=%v star=%v cards=%d", okStar, db["star"], len(svc.cards))
	}
}

func TestTaskAndShopRoutes(t *testing.T) {
	svc := New()

	// The main milestone is available at level 1; daily challenges unlock later.
	if res := svc.Handle("/game/role/main_task/gain", map[string]any{"taskID": 1000}); res["ret"] != true {
		t.Fatalf("claimMainTask failed: %v", res)
	}
	if res := svc.Handle("/game/role/main_task/gain", map[string]any{"taskID": 1001}); res["ret"] != false {
		t.Fatalf("claimed an unmet milestone: %v", res)
	}
	svc.rollTaskDay(time.Now())
	svc.level, svc.gateDaily = 15, 10
	if res := svc.Handle("/game/role/daily_task/gain", map[string]any{"taskID": 5001}); res["ret"] != true {
		t.Fatalf("claimDailyTask failed: %v", res)
	}
	if dup := svc.Handle("/game/role/daily_task/gain", map[string]any{"taskID": 5001}); dup["ret"] != false {
		t.Fatalf("expected duplicate daily task claim error, got %v", dup)
	}
	if res := svc.Handle("/game/role/daily_task/allgain", nil); res["ret"] != true {
		t.Fatalf("claimAllDailyTasks failed: %v", res)
	}
	if res := svc.Handle("/game/role/liveness/stageaward", map[string]any{"csvID": 1}); res["ret"] != false {
		t.Fatalf("claimed an unmet liveness bonus: %v", res)
	}

	// Shops
	for _, route := range []string{"/game/fixshop/get", "/game/mystery/get", "/game/frag/shop/get"} {
		if res := svc.Handle(route, map[string]any{}); res["ret"] != true {
			t.Fatalf("%s failed: %v", route, res)
		}
	}
	if res := svc.Handle("/game/fixshop/buy", map[string]any{"input": map[string]any{"idx": 3, "shopID": 32, "itemID": 5000, "count": 1}}); res["ret"] != true || svc.itemCount(5000) != 10 {
		t.Fatalf("fixshop buy failed: res=%v count=%d", res, svc.itemCount(5000))
	}
	if res := svc.Handle("/game/mystery/refresh", map[string]any{}); res["ret"] != true {
		t.Fatalf("mystery refresh failed: %v", res)
	}
	if res := svc.Handle("/game/buy_item", map[string]any{"input": map[string]any{"itemID": 502, "itemCount": 2}}); res["ret"] != true || svc.itemCount(502) != 2 {
		t.Fatalf("buy_item failed: res=%v count=%d", res, svc.itemCount(502))
	}
}

func TestExtraRoutesAndPersistence(t *testing.T) {
	dir := t.TempDir()
	store, err := playerstore.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	key := playerstore.AccountKey{Account: "shuma_100001", ServerKey: "game.cn.1"}

	svc, err := NewWithStore(store, key)
	if err != nil {
		t.Fatalf("NewWithStore: %v", err)
	}

	if res := svc.Handle("/game/role/pokedex_advance", map[string]any{"input": map[string]any{"csvID": 1}}); res["ret"] != true {
		t.Fatalf("pokedexAdvance failed: %v", res)
	}
	if res := svc.Handle("/game/talent/levelup_ready", map[string]any{"input": map[string]any{"talentID": 101}}); res["ret"] != true {
		t.Fatalf("talentLevelUpReady failed: %v", res)
	}
	if res := svc.Handle("/game/role/achievement/task/award/get", map[string]any{"input": map[string]any{"csvID": 1}}); res["ret"] != true {
		t.Fatalf("achievementTaskAward failed: %v", res)
	}
	if res := svc.Handle("/game/role/city/sprite/gift", map[string]any{"input": map[string]any{"csvID": 1}}); res["ret"] != true {
		t.Fatalf("citySpriteGift failed: %v", res)
	}
	if res := svc.Handle("/game/role/rename", map[string]any{"input": map[string]any{"name": "持久训练师"}}); res["ret"] != true {
		t.Fatalf("rename failed: %v", res)
	}

	// Re-create Service from same store to verify state persisted across restarts.
	svc2, err := NewWithStore(store, key)
	if err != nil {
		t.Fatalf("NewWithStore reload: %v", err)
	}
	if svc2.name != "持久训练师" || svc2.gold != 11500 || svc2.rmb != 1020 {
		t.Fatalf("persisted state mismatch: name=%q gold=%d rmb=%d", svc2.name, svc2.gold, svc2.rmb)
	}
}

func TestLoginSnapshotAndSyncFormat(t *testing.T) {
	svc := New()
	loginResp := svc.Handle("/game/login", nil)
	if loginResp["ret"] != true {
		t.Fatalf("login failed: %v", loginResp)
	}
	model := loginResp["model"].(map[string]any)
	roleDB := model["role"].(map[string]any)["_db"].(map[string]any)
	for _, k := range []string{"vip_sum", "vip_gift", "recharges", "ready_cards", "card_deployment", "top6_fighting_point", "trainer_skills", "house_lovers", "sign_in_gift", "draw_sum_box", "skill_point_last_recover_time", "star_skill_points"} {
		if _, ok := roleDB[k]; !ok {
			t.Fatalf("role._db missing %q", k)
		}
	}
	if gift, ok := roleDB["sign_in_gift"].([]any); !ok || len(gift) < 2 {
		t.Fatalf("role._db.sign_in_gift invalid: %#v", roleDB["sign_in_gift"])
	}

	dailyDB := model["daily_record"].(map[string]any)["_db"].(map[string]any)
	for _, k := range []string{"lianjin_gifts", "lianjin_times", "lianjin_free_times", "friend_stamina_send", "gate_times", "buy_herogate_times", "union_training_speedup", "huodong_boss_times"} {
		if _, ok := dailyDB[k]; !ok {
			t.Fatalf("daily_record._db missing %q", k)
		}
	}

	cards := model["cards"].(map[string]any)
	cardDB := cards["1001"].(map[string]any)["_db"].(map[string]any)
	attrs, ok := cardDB["attrs"].(map[string]any)
	if !ok {
		t.Fatalf("card._db.attrs missing or wrong type: %T", cardDB["attrs"])
	}
	for _, ak := range []string{"hp", "speed", "damage", "defence", "specialDamage", "specialDefence"} {
		if _, ok := attrs[ak]; !ok {
			t.Fatalf("card._db.attrs missing %q", ak)
		}
	}
	if _, ok := cardDB["nvalue_locked"].(map[string]any); !ok {
		t.Fatalf("card._db.nvalue_locked missing")
	}
	if toInt64(cardDB["effort_advance"], 0) < 1 {
		t.Fatalf("card._db.effort_advance must be >= 1, got %v", cardDB["effort_advance"])
	}
	if _, ok := cardDB["fetters"].([]any); !ok {
		t.Fatalf("card._db.fetters missing")
	}
	if _, ok := cardDB["nvalue_recast_total"]; !ok {
		t.Fatalf("card._db.nvalue_recast_total missing")
	}

	// Verify 9999999 diamond mail can be viewed and claimed.
	if mailGet := svc.Handle("/game/role/mail/get", map[string]any{"input": map[string]any{"mailID": "mail_9999999_rmb"}}); mailGet["ret"] != true {
		t.Fatalf("mail/get failed: %v", mailGet)
	}
	beforeRMB := svc.rmb
	if mailRead := svc.Handle("/game/role/mail/read", map[string]any{"input": map[string]any{"mailID": "mail_9999999_rmb"}}); mailRead["ret"] != true {
		t.Fatalf("mail/read failed: %v", mailRead)
	}
	if svc.rmb != beforeRMB+9999999 {
		t.Fatalf("expected rmb += 9999999, got %d -> %d", beforeRMB, svc.rmb)
	}

	// Verify lottery counters are maps and chat works.
	lotteryDB := model["lottery_record"].(map[string]any)["_db"].(map[string]any)
	if _, ok := lotteryDB["draw_card_up1_counters"].(map[string]any); !ok {
		t.Fatalf("draw_card_up1_counters must be map, got %T", lotteryDB["draw_card_up1_counters"])
	}
	if _, ok := lotteryDB["draw_card_up10_counters"].(map[string]any); !ok {
		t.Fatalf("draw_card_up10_counters must be map, got %T", lotteryDB["draw_card_up10_counters"])
	}
	if chatRes := svc.Handle("/game/chat", map[string]any{"input": map[string]any{"msg": "hello", "type": "world"}}); chatRes["ret"] != true {
		t.Fatalf("chat failed: %v", chatRes)
	}
	if upChoose := svc.Handle("/game/lottery/card/up/choose", map[string]any{"input": map[string]any{"choose": 1221}}); upChoose["ret"] != true {
		t.Fatalf("up/choose failed: %v", upChoose)
	}
	if upDraw := svc.Handle("/game/lottery/card/up/draw", map[string]any{"input": map[string]any{"drawType": "group_up_rmb10", "choose": 1221}}); upDraw["ret"] != true {
		t.Fatalf("up/draw failed: %v", upDraw)
	}

	// Verify card-creating routes do not put boolean true into sync.new["cards"].
	drawResp := svc.Handle("/game/lottery/card/draw", map[string]any{"input": map[string]any{"drawType": "rmb1"}})
	if syncNew := drawResp["sync"].(map[string]any)["new"].(map[string]any); syncNew["cards"] == true {
		t.Fatalf("drawCards sync.new.cards must not be boolean true")
	}
	starterResp := svc.Handle("/game/role/newbie/card/choose", map[string]any{"input": map[string]any{"cardID": 21}})
	if syncNew := starterResp["sync"].(map[string]any)["new"].(map[string]any); syncNew["cards"] == true {
		t.Fatalf("chooseStarter sync.new.cards must not be boolean true")
	}
}

func TestCardTrainingVipAndEmbattle(t *testing.T) {
	svc := New()

	// 1. Card training: nvalue lock/recast, effort train/save/advance, nature choose.
	if res := svc.Handle("/game/card/nvalue/locked/switch", map[string]any{"input": map[string]any{"cardID": "1001", "attr": "hp"}}); res["ret"] != true {
		t.Fatalf("nvalue lock failed: %v", res)
	}
	if res := svc.Handle("/game/card/nvalue/recast", map[string]any{"input": map[string]any{"cardID": "1001"}}); res["ret"] != true {
		t.Fatalf("nvalue recast failed: %v", res)
	}
	if res := svc.Handle("/game/card/effort/train", map[string]any{"input": map[string]any{"cardID": "1001", "trainType": 1, "trainTime": 1}}); res["ret"] != true {
		t.Fatalf("effort train failed: %v", res)
	}
	if res := svc.Handle("/game/card/effort/save", map[string]any{"input": map[string]any{"cardID": "1001"}}); res["ret"] != true {
		t.Fatalf("effort save failed: %v", res)
	}
	if res := svc.Handle("/game/card/effort/advance", map[string]any{"input": map[string]any{"cardID": "1001"}}); res["ret"] != true {
		t.Fatalf("effort advance failed: %v", res)
	}
	if res := svc.Handle("/game/card/nature/choose", map[string]any{"input": map[string]any{"cardID": "1001", "choose": 2}}); res["ret"] != true {
		t.Fatalf("nature choose failed: %v", res)
	}
	if res := svc.Handle("/game/card/develop", map[string]any{"input": map[string]any{"cardID": "1001", "branch": 1}}); res["ret"] != true {
		t.Fatalf("card develop failed: %v", res)
	}
	// Verify card star up with costCardIDs returns del.cards map[id]=false.
	svc.cards["1099"] = newCard("1099", 21, 1)
	starRes := svc.Handle("/game/card/star", map[string]any{"input": map[string]any{"cardID": "1001", "costCardIDs": []any{"1099"}}})
	if starRes["ret"] != true {
		t.Fatalf("card star failed: %v", starRes)
	}
	delCards := starRes["sync"].(map[string]any)["del"].(map[string]any)["cards"].(map[string]any)
	if v, ok := delCards["1099"]; !ok || v != false {
		t.Fatalf("expected sync.del.cards[\"1099\"] == false, got %#v", delCards)
	}
	svc.items[11] = int64(10)
	if res := svc.Handle("/game/card/exp/use_item", map[string]any{"input": map[string]any{"cardID": "1001", "itemID": 11, "itemCount": 10}}); res["ret"] != true {
		t.Fatalf("card exp failed: %v", res)
	}
	gateRes := svc.Handle("/game/start_gate", map[string]any{"input": map[string]any{"gateID": 10101}})
	if gateRes["ret"] != true {
		t.Fatalf("start_gate failed: %v", gateRes)
	}
	gateCard := gateRes["model"].(map[string]any)["battle"].(map[string]any)["card_attrs"].(map[string]any)["1001"].(map[string]any)
	if toInt64(gateCard["level"], 0) <= 1 {
		t.Fatalf("expected start_gate card_attrs to reflect upgraded level > 1, got %v", gateCard["level"])
	}

	// 2. VIP: buy level 0 vip gift, display switch, month/week gift.
	if res := svc.Handle("/game/role/vipgift/buy", map[string]any{"input": map[string]any{"vipLevel": 0}}); res["ret"] != true {
		t.Fatalf("vipgift buy failed: %v", res)
	}
	if res := svc.Handle("/game/role/vip/display/switch", map[string]any{"input": map[string]any{"flag": true}}); res["ret"] != true {
		t.Fatalf("vip display switch failed: %v", res)
	}
	if res := svc.Handle("/game/role/vip/month/gift", nil); res["ret"] != true {
		t.Fatalf("vip month gift failed: %v", res)
	}
	if res := svc.Handle("/game/vip/week_gift/award", map[string]any{"input": map[string]any{"type": 1}}); res["ret"] != true {
		t.Fatalf("vip week gift failed: %v", res)
	}

	// 3. Embattle: sparse map battleCardIDs + preset ready_cards + arena deploy + sync.
	svc.cards["1002"] = newCard("1002", 1, 1)
	if res := svc.Handle("/game/battle/card", map[string]any{"input": map[string]any{"battleCardIDs": map[string]any{"3": "1001", "1": "1002"}}}); res["ret"] != true {
		t.Fatalf("sparse battle/card failed: %v", res)
	}
	if len(svc.battleCards) != 2 || svc.battleCards[0] != "1002" || svc.battleCards[1] != "1001" {
		t.Fatalf("unexpected battleCards order: %#v", svc.battleCards)
	}
	if res := svc.Handle("/game/ready/card/deploy", map[string]any{"input": map[string]any{"idx": 1, "cardIDs": []any{"1001"}}}); res["ret"] != true {
		t.Fatalf("ready/card/deploy failed: %v", res)
	}
	if res := svc.Handle("/game/pw/battle/deploy", map[string]any{"input": map[string]any{"defenceCards": []any{"1001"}}}); res["ret"] != true {
		t.Fatalf("pw/battle/deploy failed: %v", res)
	}
	if res := svc.Handle("/game/deployment/sync", map[string]any{"input": map[string]any{"key": "arena_defence_cards", "flag": true}}); res["ret"] != true {
		t.Fatalf("deployment/sync failed: %v", res)
	}
}

func TestArenaRoutesAndLoginModel(t *testing.T) {
	svc := New()
	loginResp := svc.Handle("/game/login", nil)
	model := loginResp["model"].(map[string]any)
	arenaModel, ok := model["arena"].(map[string]any)
	if !ok {
		t.Fatalf("login model missing arena")
	}
	if _, ok := arenaModel["record"]; !ok {
		t.Fatalf("arena missing record")
	}
	if _, hasDB := arenaModel["_db"]; hasDB {
		t.Fatalf("arena must be flat (no _db/_mem) for REQUIRE_SYNC")
	}
	dailyDB := model["daily_record"].(map[string]any)["_db"].(map[string]any)
	for _, k := range []string{"pvp_enermys_refresh_times", "buy_pw_times", "pvp_pw_times", "pvp_pw_last_time", "buy_pw_cd_times", "item_pw_times"} {
		if _, ok := dailyDB[k]; !ok {
			t.Fatalf("daily_record._db missing %q", k)
		}
	}

	if res := svc.Handle("/game/pw/battle/get", map[string]any{"input": map[string]any{"needRefresh": true}}); res["ret"] != true {
		t.Fatalf("pw/battle/get failed: %v", res)
	}
	if res := svc.Handle("/game/pw/battle/start", map[string]any{"input": map[string]any{"battleRank": 110}}); res["ret"] != true {
		t.Fatalf("pw/battle/start failed: %v", res)
	}
	if res := svc.Handle("/game/pw/battle/end", map[string]any{"input": map[string]any{"rank": 110, "result": "win"}}); res["ret"] != true {
		t.Fatalf("pw/battle/end failed: %v", res)
	}
	if svc.arenaRank != 110 {
		t.Fatalf("expected arenaRank=110 after win, got %d", svc.arenaRank)
	}
	if res := svc.Handle("/game/pw/rank", nil); res["ret"] != true {
		t.Fatalf("pw/rank failed: %v", res)
	}
	if res := svc.Handle("/game/pw/battle/buy", nil); res["ret"] != true {
		t.Fatalf("pw/battle/buy failed: %v", res)
	}
	if res := svc.Handle("/game/pw/battle/cd/buy", nil); res["ret"] != true {
		t.Fatalf("pw/battle/cd/buy failed: %v", res)
	}
	if res := svc.Handle("/game/pw/display", map[string]any{"input": map[string]any{"card_id": 11}}); res["ret"] != true {
		t.Fatalf("pw/display failed: %v", res)
	}
}

func TestMultiAccountManagerIsolation(t *testing.T) {
	dir := t.TempDir()
	pstore, err := playerstore.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer pstore.Close()
	astore, err := login.NewAccountStore(dir)
	if err != nil {
		t.Fatalf("NewAccountStore: %v", err)
	}
	defer astore.Close()
	mgr, err := NewManager(pstore, astore, playerstore.AccountKey{Account: "shuma_100001", ServerKey: "game.cn.1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Alice registers, enters server, renames role.
	if _, _, err := astore.CheckOrRegister("alice", "none"); err != nil {
		t.Fatalf("CheckOrRegister alice: %v", err)
	}
	astore.BeginEnterServer("game.cn.1")
	loginAlice := mgr.Handle("/game/login", nil)
	if loginAlice["model"].(map[string]any)["account"].(map[string]any)["_db"].(map[string]any)["name"] != "alice" {
		t.Fatalf("expected alice account on login")
	}
	mgr.Handle("/game/role/rename", map[string]any{"input": map[string]any{"name": "爱丽丝"}})

	// Bob registers, enters server -> gets isolated save.
	if _, _, err := astore.CheckOrRegister("bob", "none"); err != nil {
		t.Fatalf("CheckOrRegister bob: %v", err)
	}
	astore.BeginEnterServer("game.cn.1")
	loginBob := mgr.Handle("/game/login", nil)
	bobRole := loginBob["model"].(map[string]any)["role"].(map[string]any)["_db"].(map[string]any)
	if bobRole["name"] == "爱丽丝" {
		t.Fatalf("expected bob to have isolated role state from alice")
	}
}

func TestChapterUnlockAndSweep(t *testing.T) {
	svc := New()
	svc.gateOpen = []int64{10101, 10102, 10103, 10104, 10105}
	start := svc.Handle("/game/start_gate", map[string]any{"input": map[string]any{"gateID": 10105}})
	if start["ret"] != true {
		t.Fatalf("start_gate 10105 failed: %v", start)
	}
	battleID := start["model"].(map[string]any)["battle"].(map[string]any)["id"].(string)
	end := svc.Handle("/game/end_gate", map[string]any{"input": map[string]any{"battleID": battleID, "gateID": 10105, "result": "win", "star": 3}})
	if end["ret"] != true {
		t.Fatalf("end_gate 10105 failed: %v", end)
	}
	updRole := end["sync"].(map[string]any)["upd"].(map[string]any)["role"].(map[string]any)["_db"].(map[string]any)
	mapOpen := updRole["map_open"].([]any)
	if len(mapOpen) < 2 || mapOpen[1] != int64(12) {
		t.Fatalf("expected map_open to unlock chapter 12, got %#v", mapOpen)
	}
	sweep := svc.Handle("/game/saodang", map[string]any{"input": map[string]any{"gateID": 10105, "times": 3}})
	if sweep["ret"] != true {
		t.Fatalf("saodang failed: %v", sweep)
	}
	resList := sweep["view"].(map[string]any)["result"].([]any)
	if len(resList) != 3 {
		t.Fatalf("expected 3 sweep results, got %d", len(resList))
	}
	firstItems := resList[0].(map[string]any)["items"].(map[any]any)
	if firstItems[int64(5000)] != int64(1) || firstItems[int64(11)] != int64(1) || firstItems["gold"] == nil {
		t.Fatalf("expected int item keys 5000/11 and gold in sweep items, got %#v", firstItems)
	}
}

func TestDeploymentSyncAndHuodongShow(t *testing.T) {
	svc := New()
	loginResp := svc.Handle("/game/login", nil)
	roleDB := loginResp["model"].(map[string]any)["role"].(map[string]any)["_db"].(map[string]any)
	if _, ok := roleDB["deployments_sync"].(map[string]any); !ok {
		t.Fatalf("login role._db missing deployments_sync")
	}

	syncResp := svc.Handle("/game/deployment/sync", map[string]any{"input": map[string]any{"key": "arena_defence_cards", "flag": true}})
	if syncResp["ret"] != true {
		t.Fatalf("deployment/sync failed: %v", syncResp)
	}
	updRole := syncResp["sync"].(map[string]any)["upd"].(map[string]any)["role"].(map[string]any)["_db"].(map[string]any)
	flags := updRole["deployments_sync"].(map[string]any)
	if flags["arena_defence_cards"] != true {
		t.Fatalf("expected deployments_sync[arena_defence_cards]=true, got %#v", flags)
	}

	huodong := svc.Handle("/game/huodong/show", nil)
	if huodong["ret"] != true {
		t.Fatalf("huodong/show failed: %v", huodong)
	}
	view := huodong["view"].(map[string]any)
	day, ok := view["day"].(int64)
	if !ok || day < 1 || day > 7 {
		t.Fatalf("expected valid day 1..7, got %#v", view["day"])
	}
	openMap := view["open"].(map[int64]any)
	if openMap[1] != int64(1) {
		t.Fatalf("expected huodong 1 open, got %#v", openMap)
	}
}

func TestCardAttrs2AndLevelScaling(t *testing.T) {
	svc := New()
	svc.addItem(11, 20)
	cardDB := svc.cards["1001"].(map[string]any)["_db"].(map[string]any)
	attrs2, ok := cardDB["attrs2"].(map[string]any)
	if !ok || attrs2["strike"] == nil {
		t.Fatalf("newCard missing attrs2: %#v", cardDB["attrs2"])
	}
	hpBefore := cardDB["attrs"].(map[string]any)["hp"].(int64)
	fpBefore := toInt64(cardDB["fighting_point"], 0)
	strikeBefore := attrs2["strike"].(int64)

	res := svc.Handle("/game/card/exp/use_items", map[string]any{
		"input": map[string]any{
			"cardID": "1001",
			"items":  map[any]any{int64(11): int64(10)},
		},
	})
	if res["ret"] != true {
		t.Fatalf("exp/use_items failed: %v", res)
	}
	lvAfter := toInt64(cardDB["level"], 1)
	hpAfter := cardDB["attrs"].(map[string]any)["hp"].(int64)
	fpAfter := toInt64(cardDB["fighting_point"], 0)
	strikeAfter := cardDB["attrs2"].(map[string]any)["strike"].(int64)
	if lvAfter <= 1 || hpAfter <= hpBefore || fpAfter <= fpBefore || strikeAfter <= strikeBefore {
		t.Fatalf("expected level/attrs/attrs2/fp to increase: lv=%d hp=%d->%d fp=%d->%d strike=%d->%d",
			lvAfter, hpBefore, hpAfter, fpBefore, fpAfter, strikeBefore, strikeAfter)
	}
}
