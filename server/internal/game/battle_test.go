package game

import (
	"slices"
	"testing"
	"time"
)

func openBattle(t *testing.T, svc *Service, gate int64) string {
	t.Helper()
	reply := svc.Handle("/game/start_gate", map[string]any{"gateID": gate})
	if reply["ret"] != true {
		t.Fatalf("start %d: %v", gate, reply)
	}
	return reply["model"].(map[string]any)["battle"].(map[string]any)["id"].(string)
}

func settle(t *testing.T, svc *Service, id string, gate int64, result string, stars int64) {
	t.Helper()
	reply := svc.Handle("/game/end_gate", map[string]any{"battleID": id, "gateID": gate, "result": result, "star": stars})
	if reply["ret"] != true || reply["view"].(map[string]any)["result"] != result {
		t.Fatalf("settle %s: %v", result, reply)
	}
}

func TestGateStartAndSettlement(t *testing.T) {
	svc := New()
	if got := svc.Handle("/game/start_gate", map[string]any{"gateID": 10102}); got["ret"] != false {
		t.Fatalf("locked gate started: %v", got)
	}
	id := openBattle(t, svc, 10101)
	if retry := openBattle(t, svc, 10101); retry != id || svc.stamina != 114 {
		t.Fatalf("start retry was charged again: %d", svc.stamina)
	}
	if got := svc.Handle("/game/end_gate", map[string]any{"gateID": 10101, "result": "win", "star": 3}); got["ret"] != false {
		t.Fatalf("accepted missing battle ID: %v", got)
	}
	settle(t, svc, id, 10101, "fail", 0)
	if svc.gold != 10000 || len(svc.gateStar) != 0 || svc.level != 1 || len(svc.gateOpen) != 1 {
		t.Fatal("failure granted progression or reward")
	}
	id = openBattle(t, svc, 10101)
	settle(t, svc, id, 10101, "win", 3)
	settle(t, svc, id, 10101, "win", 3)
	if svc.gold != 10875 || svc.level != 2 || svc.stamina != 108 || !slices.Contains(svc.gateOpen, int64(10102)) {
		t.Fatal("win must update the role exactly once")
	}
	replay := openBattle(t, svc, 10101)
	settle(t, svc, replay, 10101, "win", 1)
	if svc.gateStar[10101].stars != 3 || svc.gateSnapshot()["gate_star_sum"] != int64(3) {
		t.Fatal("replaying with fewer stars lowered the record")
	}

	id = openBattle(t, svc, 10102)
	svc.activeGate.expires = time.Now().Add(-time.Second)
	if got := svc.Handle("/game/end_gate", map[string]any{"battleID": id, "gateID": 10102, "result": "win", "star": 3}); got["ret"] != false {
		t.Fatalf("expired battle settled: %v", got)
	}
	svc.stamina = 5
	if got := svc.Handle("/game/start_gate", map[string]any{"gateID": 10102}); got["ret"] != false || svc.stamina != 5 {
		t.Fatalf("allowed battle with insufficient stamina: %v", got)
	}
}

func TestGateChestAndChapterRewards(t *testing.T) {
	svc := New()
	req := map[string]any{"mapID": 11, "awardLevel": 1}
	if got := svc.Handle("/game/role/map/star_award", req); got["ret"] != false {
		t.Fatalf("unearned reward: %v", got)
	}
	for _, gate := range []int64{10101, 10102} {
		settle(t, svc, openBattle(t, svc, gate), gate, "win", 3)
	}
	for range 2 {
		if got := svc.Handle("/game/role/map/star_award", req); got["ret"] != true {
			t.Fatalf("chapter reward: %v", got)
		}
		if got := svc.Handle("/game/role/gate/award", map[string]any{"gateID": 10102, "type": "chest"}); got["ret"] != true {
			t.Fatalf("chest reward: %v", got)
		}
	}
	if svc.gold != 37750 || svc.rmb != 1070 || svc.items[5000] != int64(15) || svc.frags[21351] != int64(20) {
		t.Fatal("reward totals diverged on retry")
	}
}
