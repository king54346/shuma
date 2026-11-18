package game

import (
	"testing"
	"time"
)

func TestSignInRestoresModelAndGrantsOnce(t *testing.T) {
	svc := New()
	model := svc.Handle("/game/login", nil)["model"].(map[string]any)
	role := model["role"].(map[string]any)["_db"].(map[string]any)
	month := model["monthly_record"].(map[string]any)["_db"].(map[string]any)
	if role["sign_in_gift"].([]any)[0] != int64(1) || month["month"].(int64) < 200001 {
		t.Fatalf("missing valid sign-in defaults: %v", month)
	}
	for range 2 {
		if reply := svc.Handle("/game/role/sign_in", nil); reply["ret"] != true {
			t.Fatalf("daily sign-in: %v", reply)
		}
	}
	if svc.sign.count != 1 || svc.sign.total != 1 || svc.rmb != 1100 {
		t.Fatalf("duplicate sign-in changed state: count=%d, rmb=%d", svc.sign.count, svc.rmb)
	}
	svc.sign.rollMonth(time.Now().In(signZone).AddDate(0, 0, 32))
	if svc.sign.total != 1 || svc.sign.count != 0 {
		t.Fatal("month rollover lost lifetime progress or kept monthly progress")
	}
}

func TestSignInTotalAwardAndBuy(t *testing.T) {
	svc := New()
	svc.sign.rollMonth(time.Now().In(signZone))
	svc.sign.total = 5
	svc.sign.claimed = 0

	reply := svc.Handle("/game/role/sign_in/total_award", nil)
	if reply["ret"] != true {
		t.Fatalf("total_award failed: %v", reply)
	}
	if svc.sign.claimed != 1 {
		t.Fatalf("expected claimed=1, got %d", svc.sign.claimed)
	}
	syncMap := reply["sync"].(map[string]any)
	updRole := syncMap["upd"].(map[string]any)["role"].(map[string]any)["_db"].(map[string]any)
	gift := updRole["sign_in_gift"].([]any)
	if gift[0] != int64(2) || gift[1] != 0 {
		t.Fatalf("expected next sign_in_gift [2, 0], got %#v", gift)
	}
	newRole := syncMap["new"].(map[string]any)["role"].(map[string]any)["_db"].(map[string]any)
	if newRole["sign_in_gift"] != true {
		t.Fatalf("expected sync.new.role._db.sign_in_gift=true")
	}

	// VIP double-claim with csvID > 0 should succeed.
	if vipReply := svc.Handle("/game/role/sign_in/buy", map[string]any{"input": map[string]any{"csvID": 1}}); vipReply["ret"] != true {
		t.Fatalf("vip double claim failed: %v", vipReply)
	}
}
