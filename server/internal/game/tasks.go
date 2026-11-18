package game

import (
	"maps"
	"time"
)

var livenessRewards = [...]gateReward{
	{},
	{gold: 10000, items: map[int64]int64{930: 5, 5000: 5}},
	{gold: 10000, items: map[int64]int64{13: 20, 453: 30}},
	{gold: 20000, items: map[int64]int64{900: 5, 901: 5, 523: 3}},
	{gold: 20000, items: map[int64]int64{519: 1, 453: 50}},
	{gold: 50000, rmb: 50, items: map[int64]int64{19: 2}},
}

const normalGateDaily int64 = 5001

func (p *Service) rollTaskDay(now time.Time) {
	d := now.In(signZone)
	day := int64(d.Year()*10000 + int(d.Month())*100 + d.Day())
	if p.taskDay != day {
		p.taskDay = day
		p.dailyTasks = make(map[int64]int64)
		p.livenessAwards = make(map[int64]any)
		p.liveness = 0
		p.gateDaily = 0
	}
}

func (p *Service) tasksSnapshot() map[string]any {
	var flags int64
	if state, tracked := p.dailyTasks[normalGateDaily]; tracked {
		flags = state
	} else if p.gateDaily >= 10 {
		flags = 1
	}
	daily := map[int64]any{normalGateDaily: flags}
	main := map[int64]any{}
	for i := range mainLevelRewards {
		id := 1000 + int64(i)
		if !p.mainTasksClaimed[id] {
			var flag int64
			if p.level >= int64(i+1) {
				flag = 1
			}
			main[1000] = []any{id, flag}
			break
		}
	}
	if len(main) == 0 {
		last := 1000 + int64(len(mainLevelRewards)-1)
		main[1000] = []any{last, 2}
	}
	return map[string]any{"daily": daily, "main": main}
}

func (p *Service) taskSync(role map[string]any) map[string]any {
	reply := p.roleSync(role)
	models := reply["sync"].(map[string]any)["upd"].(map[string]any)
	models["tasks"] = map[string]any{"_mem": p.tasksSnapshot()}
	models["daily_record"] = map[string]any{"_db": map[string]any{
		"liveness_point": p.liveness, "liveness_stage_award": maps.Clone(p.livenessAwards),
		"gate_chanllenge": p.gateDaily,
	}}
	return reply
}

func (p *Service) normalDailyClaimable() bool {
	state, handled := p.dailyTasks[normalGateDaily]
	return p.level >= 15 && p.gateDaily >= 10 && (!handled || state == 1)
}

func (p *Service) claimDailyTask(req any) map[string]any {
	if toInt64(getInputMap(req)["taskID"], 0) != normalGateDaily {
		return gameError("invalid_task")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rollTaskDay(time.Now())
	if !p.normalDailyClaimable() {
		return gameError("task_unavailable")
	}
	return p.claimNormalDaily()
}

func (p *Service) claimAllDailyTasks(_ any) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rollTaskDay(time.Now())
	if !p.normalDailyClaimable() {
		reply := p.taskSync(map[string]any{})
		reply["view"] = map[string]any{"result": map[string]any{}}
		return reply
	}
	return p.claimNormalDaily()
}

func (p *Service) claimNormalDaily() map[string]any {
	p.dailyTasks[normalGateDaily] = 2
	p.liveness += 10
	p.addRoleExp(150)
	fields := map[string]any{"level": p.level, "level_exp": p.levelExp}
	view := p.applyReward(gateReward{gold: 8000}, false, fields)
	view["role_exp"] = 150
	reply := p.taskSync(fields)
	reply["view"] = view
	return reply
}

func (p *Service) claimMainTask(req any) map[string]any {
	id := toInt64(getInputMap(req)["taskID"], 0)
	idx := id - 1000
	if idx < 0 || idx >= int64(len(mainLevelRewards)) {
		return gameError("invalid_task")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mainTasksClaimed[id] || p.level < idx+1 || (idx > 0 && !p.mainTasksClaimed[id-1]) {
		return gameError("task_unavailable")
	}
	p.mainTasksClaimed[id] = true
	fields := map[string]any{}
	view := p.applyReward(mainLevelRewards[idx], false, fields)
	reply := p.taskSync(fields)
	reply["view"] = view
	return reply
}

func (p *Service) claimLivenessAward(req any) map[string]any {
	id := toInt64(getInputMap(req)["csvID"], 0)
	if id < 1 || id >= int64(len(livenessRewards)) {
		return gameError("invalid_liveness")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rollTaskDay(time.Now())
	if _, claimed := p.livenessAwards[id]; claimed || p.liveness < id*20 {
		return gameError("liveness_unavailable")
	}
	p.livenessAwards[id] = 1
	fields := map[string]any{}
	view := p.applyReward(livenessRewards[id], false, fields)
	reply := p.taskSync(fields)
	reply["view"] = view
	return reply
}

func (p *Service) trainerDailyAward(_ any) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.trainerGiftTimes > 0 {
		return gameError("already_claimed")
	}
	p.trainerGiftTimes = 1
	fields := map[string]any{}
	view := p.applyReward(gateReward{gold: 20000, items: map[int64]int64{501: 10}}, false, fields)
	reply := p.taskSync(fields)
	upd := reply["sync"].(map[string]any)["upd"].(map[string]any)
	upd["daily_record"] = map[string]any{
		"_db": map[string]any{
			"trainer_gift_times": p.trainerGiftTimes,
		},
	}
	reply["view"] = view
	return reply
}
