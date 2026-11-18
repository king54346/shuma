package game

import (
	"log"
	"slices"
	"sync"
	"time"

	"shuma-server/internal/playerstore"
)

type Service struct {
	requestMu           sync.Mutex
	savedVersion        int64
	mu                  sync.Mutex
	store               *playerstore.Store
	storeKey            playerstore.AccountKey
	accountID           int64
	accountName         string
	name                string
	figure              int64
	newbieGuide         []int64
	guideAwards         map[int64]string
	gold                int64
	rmb                 int64
	items               map[int64]any
	frags               map[int64]any
	houseLovers         map[int64]lover
	sign                signState
	cards               map[string]any
	nextCardID          int64
	battleCards         []string
	gateStar            map[int64]gateProgress
	gateOpen            []int64
	gateClaims          map[int64]bool
	level               int64
	levelExp            int64
	stamina             int64
	staminaAt           int64
	buyStaminaTimes     int64
	lianjinTimes        int64
	lianjinGifts        map[int64]int64
	dailyTasks          map[int64]int64
	mainTasksClaimed    map[int64]bool
	livenessAwards      map[int64]any
	liveness            int64
	taskDay             int64
	gateDaily           int64
	pokedexAdv          map[int64]any
	registeredSpecies   map[int64]int64
	draws               drawState
	shops               map[string]shopState
	fragmentCoins       int64
	talents             map[int64]any
	talentPoint         int64
	achievementTasks    map[int64]any
	achievementBoxes    []int64
	citySpriteGifts     int64
	vipLevel            int64
	vipSum              int64
	vipHide             bool
	vipGift             map[int64]int64
	vipMonthGiftClaimed bool
	vipWeekGifts        map[int64]int64
	trainerGiftTimes    int64
	readyCards          map[int64]readyTeam
	arenaDefenceCards   []string
	deploymentsSync     map[string]bool
	pendingEffort       map[string]map[string]int64
	arenaRank           int64
	arenaRankTop        int64
	arenaDisplay        int64
	arenaResultPoint    int64
	pvpRefreshTimes     int64
	pvpPwTimes          int64
	pvpPwLastTime       int64
	buyPwTimes          int64
	buyPwCdTimes        int64
	itemPwTimes         int64
	pwRankAwards        map[int64]int64
	pwPointAwards       map[int64]int64
	mailbox             []mailItem
	readMailbox         []mailItem
	chatMessages        map[string][]chatMsg
	nextChatID          int64
	drawUpChoose        int64
	activeGate          *gateBattle
	settled             map[string]gateSettlement
	syncVersion         int64
}

func New() *Service {
	return &Service{
		accountID:        100001,
		accountName:      "shuma_100001",
		name:             "本地测试训练师",
		figure:           1,
		newbieGuide:      []int64{-2, -1, 1, 2},
		guideAwards:      make(map[int64]string),
		gold:             10000,
		rmb:              1000,
		items:            make(map[int64]any),
		frags:            make(map[int64]any),
		houseLovers:      make(map[int64]lover),
		nextCardID:       1002,
		cards:            map[string]any{"1001": newCard("1001", 21, 1)},
		battleCards:      []string{"1001"},
		gateStar:         make(map[int64]gateProgress),
		gateOpen:         []int64{10101},
		gateClaims:       make(map[int64]bool),
		level:            1,
		stamina:          120,
		staminaAt:        time.Now().Unix(),
		dailyTasks:       make(map[int64]int64),
		mainTasksClaimed: make(map[int64]bool),
		livenessAwards:   make(map[int64]any),
		pokedexAdv:       make(map[int64]any),
		shops:            make(map[string]shopState),
		talents:          make(map[int64]any),
		talentPoint:      10,
		achievementTasks: make(map[int64]any),
		vipGift:          make(map[int64]int64),
		vipWeekGifts:     make(map[int64]int64),
		lianjinGifts:     make(map[int64]int64),
		readyCards:       make(map[int64]readyTeam),
		deploymentsSync:  make(map[string]bool),
		pendingEffort:    make(map[string]map[string]int64),
		arenaRank:        120,
		arenaRankTop:     120,
		arenaDisplay:     21,
		pwRankAwards:     make(map[int64]int64),
		pwPointAwards:    make(map[int64]int64),
		mailbox:          defaultMailbox(),
		chatMessages:     defaultChatMessages(),
		nextChatID:       2,
		settled:          make(map[string]gateSettlement),
		syncVersion:      1,
	}
}

func (p *Service) addGuideID(id int64) []any {
	found := false
	for _, g := range p.newbieGuide {
		if g == id {
			found = true
			break
		}
	}
	if !found {
		p.newbieGuide = append(p.newbieGuide, id)
	}
	out := make([]any, len(p.newbieGuide))
	for i, g := range p.newbieGuide {
		out[i] = g
	}
	return out
}

func getInputMap(req any) map[string]any {
	if m, ok := req.(map[string]any); ok {
		if inp, ok := m["input"].(map[string]any); ok {
			return inp
		}
		return m
	}
	return map[string]any{}
}

func toInt64(v any, def int64) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int64:
		return x
	case uint64:
		return int64(x)
	case float64:
		return int64(x)
	default:
		return def
	}
}

func (p *Service) Handle(route string, req any) map[string]any {
	if p.store == nil {
		return p.dispatch(route, req)
	}
	p.requestMu.Lock()
	defer p.requestMu.Unlock()
	if err := p.persistIfChanged(); err != nil {
		log.Printf("[game] pending save failed: %v", err)
		return gameError("save_unavailable")
	}
	resp := p.dispatch(route, req)
	if err := p.persistIfChanged(); err != nil {
		log.Printf("[game] save failed: %v", err)
		return gameError("save_unavailable")
	}
	return resp
}

func (p *Service) dispatch(route string, req any) map[string]any {
	switch route {
	case "/game/login":
		return p.login()
	case "/game/sync":
		return map[string]any{"ret": true, "server_time": time.Now().Unix()}
	case "/game/role/guide/newbie":
		return p.saveGuide(req)
	case "/game/role/guide/newbie/award":
		return p.claimGuideAward(req)
	case "/game/role/newbie/init":
		return p.initRole(req)
	case "/game/role/newbie/card/choose":
		return p.chooseStarter(req)
	case "/game/battle/card":
		return p.setBattleCards(req)
	case "/game/ready/card/deploy":
		return p.deployReadyCards(req)
	case "/game/pw/battle/deploy":
		return p.deployArenaCards(req)
	case "/game/pw/battle/get":
		return p.pwBattleGet(req)
	case "/game/pw/battle/start":
		return p.pwBattleStart(req)
	case "/game/pw/battle/end":
		return p.pwBattleEnd(req)
	case "/game/pw/battle/pass", "/game/pw/battle/onekey":
		return p.pwBattlePass(req)
	case "/game/pw/battle/buy":
		return p.pwBattleBuy(req)
	case "/game/pw/battle/cd/buy":
		return p.pwBattleCDBuy(req)
	case "/game/pw/battle/item/use":
		return p.pwBattleItemUse(req)
	case "/game/pw/display":
		return p.pwDisplay(req)
	case "/game/pw/rank":
		return p.pwRank(req)
	case "/game/pw/role/info":
		return p.pwRoleInfo(req)
	case "/game/pw/battle/rank/award":
		return p.pwRankAward(req)
	case "/game/pw/battle/point/award":
		return p.pwPointAward(req)
	case "/game/pw/shop/buy":
		return p.pwShopBuy(req)
	case "/game/pw/playrecord/get":
		return map[string]any{"ret": true, "view": map[string]any{"play": map[string]any{}}}
	case "/game/huodong/show":
		return p.huodongShow(req)
	case "/game/huodong/start":
		return p.huodongStart(req)
	case "/game/huodong/end":
		return p.huodongEnd(req)
	case "/game/huodong/saodang":
		return p.huodongSaodang(req)
	case "/game/huodong/card":
		return p.setBattleCards(req)
	case "/game/deployment/sync":
		return p.syncDeployment(req)
	case "/game/start_gate":
		return p.startGate(req)
	case "/game/end_gate":
		return p.endGate(req)
	case "/game/saodang":
		return p.sweepGate(req)
	case "/game/role/gate/award":
		return p.claimGateChest(req)
	case "/game/role/map/star_award":
		return p.claimChapterStars(req)
	case "/game/lover/add/feels":
		return p.giftLover(req)
	case "/game/lover/bless/card":
		return p.blessCard(req)
	case "/game/role/sign_in", "/game/role/sign_in/buy",
		"/game/role/sign_in/total_award", "/game/role/sign_in/month/total_award":
		return p.signIn(route, req)
	case "/game/lottery/card/draw", "/game/lottery/card/up/draw":
		return p.drawCards(req)
	case "/game/lottery/card/up/choose":
		return p.chooseDrawUp(req)
	case "/game/rank":
		return p.localRank(req)
	case "/game/society/friend/list", "/game/society/friend/search", "/game/society/friend/online/list":
		return listView("roles", []any{})
	case "/game/union/rank":
		return listView("unions", []any{})
	case "/game/role/item/use":
		return p.useItems(req)
	case "/game/role/item/sell":
		return p.sellItems(req)
	case "/game/role/frag/sell_many":
		return p.sellFrags(req)
	case "/game/role/frag/comb":
		return p.combineFrag(req)
	case "/game/role/stamina/use_item":
		return p.useStaminaItem(req)
	case "/game/role/stamina/buy":
		return p.buyStamina(req)
	case "/game/role/lianjin":
		return p.lianjin(req)
	case "/game/role/lianjin/total_award":
		return p.lianjinTotalAward(req)
	case "/game/role/rename":
		return p.renameRole(req)
	case "/game/role/figure":
		return p.switchFigure(req)
	case "/game/card/exp/use_item":
		return p.cardExpUseItem(req)
	case "/game/card/exp/use_items":
		return p.cardExpUseItems(req)
	case "/game/card/advance":
		return p.cardAdvance(req)
	case "/game/card/star":
		return p.cardStarUp(req)
	case "/game/card/skill/level/up":
		return p.cardSkillLevelUp(req)
	case "/game/card/locked/switch":
		return p.cardLockSwitch(req)
	case "/game/card/nvalue/locked/switch":
		return p.cardNValueLockSwitch(req)
	case "/game/card/nvalue/recast":
		return p.cardNValueRecast(req)
	case "/game/card/effort/train":
		return p.cardEffortTrain(req)
	case "/game/card/effort/save":
		return p.cardEffortSave(req)
	case "/game/card/effort/advance":
		return p.cardEffortAdvance(req)
	case "/game/card/nature/choose":
		return p.cardNatureChoose(req)
	case "/game/card/develop":
		return p.cardDevelop(req)
	case "/game/equip/strength", "/game/equip/strength/exp":
		return p.equipUpgrade(req, "strength")
	case "/game/equip/advance":
		return p.equipUpgrade(req, "advance")
	case "/game/equip/star":
		return p.equipUpgrade(req, "star")
	case "/game/role/daily_task/gain":
		return p.claimDailyTask(req)
	case "/game/role/daily_task/allgain":
		return p.claimAllDailyTasks(req)
	case "/game/role/main_task/gain":
		return p.claimMainTask(req)
	case "/game/role/liveness/stageaward":
		return p.claimLivenessAward(req)
	case "/game/fixshop/get":
		return p.shopGet("fix_shop")
	case "/game/fixshop/refresh":
		return p.shopRefresh("fix_shop")
	case "/game/fixshop/buy":
		return p.shopBuy(req, "fix_shop")
	case "/game/mystery/get":
		return p.shopGet("mystery_shop")
	case "/game/mystery/refresh":
		return p.shopRefresh("mystery_shop")
	case "/game/mystery/buy":
		return p.shopBuy(req, "mystery_shop")
	case "/game/frag/shop/get":
		return p.shopGet("frag_shop")
	case "/game/frag/shop/refresh":
		return p.shopRefresh("frag_shop")
	case "/game/frag/shop/buy":
		return p.shopBuy(req, "frag_shop")
	case "/game/buy_item":
		return p.buyItem(req)
	case "/game/role/pokedex_advance":
		return p.pokedexAdvance(req)
	case "/game/talent/levelup_ready":
		return p.talentLevelUpReady(req)
	case "/game/talent/levelup_end":
		return p.talentLevelUpEnd(req)
	case "/game/trainer/daily/award":
		return p.trainerDailyAward(req)
	case "/game/talent/reset":
		return p.talentReset(req)
	case "/game/role/achievement/task/award/get":
		return p.achievementTaskAward(req)
	case "/game/role/achievement/box/award/get":
		return p.achievementBoxAward(req)
	case "/game/role/city/sprite/active":
		return map[string]any{"ret": true}
	case "/game/role/city/sprite/gift":
		return p.citySpriteGift(req)
	case "/game/role/vipgift/buy":
		return p.buyVipGift(req)
	case "/game/role/vip/display/switch":
		return p.switchVipDisplay(req)
	case "/game/role/vip/month/gift":
		return p.claimVipMonthGift(req)
	case "/game/vip/week_gift/award":
		return p.claimVipWeekGift(req)
	case "/game/role/mail/get":
		return p.mailGet(req)
	case "/game/role/mail/read":
		return p.mailRead(req)
	case "/game/role/mail/read/all":
		return p.mailReadAll(req)
	case "/game/role/mail/delete":
		return p.mailDelete(req)
	case "/game/chat":
		return p.sendChat(req)
	case "/game/chat/del":
		return map[string]any{"ret": true}
	default:
		log.Printf("[game] Unhandled route %s", route)
		return map[string]any{"ret": true}
	}
}

// roleSync is called with p.mu held. Callers take copies of mutable data.
func (p *Service) roleSync(fields map[string]any) map[string]any {
	return map[string]any{
		"ret": true,
		"sync": map[string]any{
			"version": p.syncVersion,
			"upd":     map[string]any{"role": map[string]any{"_db": fields}},
		},
	}
}

func gameError(code string) map[string]any {
	return map[string]any{"ret": false, "err": code}
}

// cardIDs is called with p.mu held. Local card IDs are decimal strings.
func (p *Service) cardIDs() []any {
	ids := make([]string, 0, len(p.cards))
	for id := range p.cards {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	result := make([]any, len(ids))
	for i, id := range ids {
		result[i] = id
	}
	return result
}
