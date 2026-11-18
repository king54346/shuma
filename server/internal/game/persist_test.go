package game

import (
	"shuma-server/internal/playerstore"
	"testing"
)

func TestClaimRecordsSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	key := playerstore.AccountKey{Account: "local", ServerKey: "game.cn.1"}
	load := func() *Service {
		t.Helper()
		store, err := playerstore.NewStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = store.Close() })
		p, err := NewWithStore(store, key)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := load()
	call := func(s *Service, route string, req any) {
		t.Helper()
		if resp := s.Handle(route, req); resp["ret"] != true {
			t.Fatalf("%s: %v", route, resp)
		}
	}
	call(p, "/game/role/sign_in", nil)
	guide := map[string]any{"guideCsvID": 103}
	call(p, "/game/role/guide/newbie/award", guide)
	for _, gate := range []int64{10101, 10102} {
		settle(t, p, openBattle(t, p, gate), gate, "win", 3)
	}
	chapter := map[string]any{"mapID": 11, "awardLevel": 1}
	call(p, "/game/role/map/star_award", chapter)

	q := load()
	call(q, "/game/role/sign_in", nil)
	call(q, "/game/role/guide/newbie/award", guide)
	call(q, "/game/role/map/star_award", chapter)
	if q.gold != p.gold || q.rmb != p.rmb || q.sign.total != 1 || len(q.cards) != 2 || q.frags[21351] != int64(20) {
		t.Fatalf("reopened save changed rewards: gold=%d diamonds=%d signs=%d cards=%d fragments=%v",
			q.gold, q.rmb, q.sign.total, len(q.cards), q.frags)
	}
}
