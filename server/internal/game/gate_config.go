package game

// Values are from scene_conf, world_map, items and base_attribute.role_level.
const (
	firstGate            int64 = 10101
	lastFirstChapterGate int64 = 10105
	firstChapter         int64 = 11
	firstChapterStamina  int64 = 6
	firstChapterExp      int64 = 6
	firstChapterGold     int64 = 875
)

func chapterForGate(gate int64) int64 {
	switch {
	case gate >= 10101 && gate <= 12210:
		return 10 + (gate-10000)/100
	case gate >= 20101 && gate <= 22205:
		return 110 + (gate-20000)/100
	case gate >= 30101 && gate <= 32202:
		return 210 + (gate-30000)/100
	default:
		return 0
	}
}

func chapterStageCount(ch int64) int64 {
	switch {
	case ch == 11:
		return 5
	case ch == 12:
		return 6
	case ch == 13 || ch == 14:
		return 8
	case ch >= 15 && ch <= 32:
		return 10
	case ch >= 111 && ch <= 115:
		return 3
	case ch >= 116 && ch <= 132:
		return 5
	case ch >= 211 && ch <= 232:
		return 2
	default:
		return 0
	}
}

func isValidNormalGate(gate int64) bool {
	ch := chapterForGate(gate)
	n := chapterStageCount(ch)
	if n == 0 {
		return false
	}
	stage := gate % 100
	return stage >= 1 && stage <= n
}

func nextNormalGate(gate int64) int64 {
	ch := chapterForGate(gate)
	n := chapterStageCount(ch)
	if n == 0 {
		return 0
	}
	stage := gate % 100
	if stage < n {
		return gate + 1
	}
	if ch < 32 {
		return 10000 + (ch+1-10)*100 + 1
	}
	if ch >= 111 && ch < 132 {
		return 20000 + (ch+1-110)*100 + 1
	}
	if ch >= 211 && ch < 232 {
		return 30000 + (ch+1-210)*100 + 1
	}
	return 0
}

type gateReward struct {
	gold, rmb    int64
	items, frags map[int64]int64
}

var firstChapterChests = map[int64]gateReward{
	10102: {gold: 20000, rmb: 20, items: map[int64]int64{5000: 10}},
	10104: {rmb: 10, items: map[int64]int64{5001: 1, 5002: 1, 5003: 1}},
}

var chapterStarRewards = [...]struct {
	minStars int64
	reward   gateReward
}{
	{},
	{6, gateReward{gold: 6000, rmb: 50, items: map[int64]int64{5000: 5}, frags: map[int64]int64{21351: 20}}},
	{15, gateReward{gold: 6000, rmb: 50, items: map[int64]int64{5000: 5, 519: 1}}},
}

var roleLevelExp = [...]int64{
	6, 6, 6, 6, 6, 6, 6, 12, 12, 12, 18, 30,
	40, 50, 60, 70, 80, 90, 100, 120, 150, 190, 250, 320,
	360, 400, 450, 500, 550, 600, 640, 670, 690, 710, 730, 780,
	820, 860, 890, 920, 1020, 1120, 1220, 1320, 1420, 1520, 1620, 1720,
	1820, 1920, 1960, 2160, 2360, 2560, 2760, 3060, 3360, 3660, 3960, 4260,
	4600, 4940, 5280, 5620, 5960, 6760, 7560, 8360, 9160, 9960, 10800, 11640,
	12480, 13320, 14160, 15000, 15840, 16680, 17520, 18360, 18840, 19320, 19800, 20280,
	20760, 21240, 21720, 22200, 22680, 23160, 23700, 24240, 24780, 25320, 26070, 26820,
	27570, 28320, 29070, 30270, 31170, 32070, 32970, 33870, 35370, 36870, 38370, 39870,
	41370, 42870, 44370, 45870, 47370, 48870, 50370, 51870, 53370, 54870, 56370, 57870,
	59370, 60870, 62370, 63870,
}
