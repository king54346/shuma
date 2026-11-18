package main

import (
	"context"
	"net"
	"testing"
	"time"

	"shuma-server/internal/game"
	"shuma-server/internal/playerstore"
	"shuma-server/internal/protocol"
	"shuma-server/internal/tcpserver"
)

func TestMenuWorkflowOverWANetTask(t *testing.T) {
	dir := t.TempDir()
	key := playerstore.AccountKey{Account: "menu-test", ServerKey: "game.cn.1"}
	store, err := playerstore.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.Update(key, func(save *playerstore.PlayerSave) error {
		save.Items[11] = 3
		save.Fragments = map[int64]int64{20011: 30}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	svc, err := game.NewWithStore(store, key)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tcpserver.Serve(ctx, ln, "menu-test", svc.Handle) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("test listener did not shut down")
		}
	}()
	conn, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	seq := uint16(0)
	send := func(route string, input any, wantOK bool) map[string]any {
		t.Helper()
		seq++
		frame, err := protocol.PackAckPacket(route, seq, map[string]any{"input": input}, protocol.InitialKey())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Write(frame); err != nil {
			t.Fatal(err)
		}
		pkt, err := protocol.ReadPacket(conn, protocol.InitialKey())
		if err != nil {
			t.Fatal(err)
		}
		reply, _ := pkt.Data.(map[string]any)
		if pkt.URL != route || pkt.SynID != seq || reply["ret"] != wantOK {
			t.Fatalf("%s: %v", route, pkt)
		}
		return reply
	}

	login := send("/game/login", nil, true)
	model := login["model"].(map[string]any)
	role := model["role"].(map[string]any)["_db"].(map[string]any)
	deployment := role["card_deployment"].(map[string]any)
	for _, key := range []string{"union_training", "arena", "craft", "cross_craft", "cross_arena", "gym", "cross_mine", "cross_supremacy"} {
		if _, ok := deployment[key].(map[string]any); !ok {
			t.Fatalf("missing deployment %s", key)
		}
	}
	card := model["cards"].(map[string]any)["1001"].(map[string]any)["_db"].(map[string]any)
	if _, ok := card["badge_guard"].([]any); !ok {
		t.Fatalf("missing badge guard array: %v", card)
	}
	send("/game/fixshop/get", nil, true)
	send("/game/fixshop/buy", map[string]any{"idx": 3, "shopID": 32, "itemID": 5000}, true)
	send("/game/fixshop/buy", map[string]any{"idx": 3, "shopID": 32, "itemID": 11}, false)
	send("/game/lottery/card/draw", map[string]any{"drawType": "free1"}, true)
	send("/game/lottery/card/draw", map[string]any{"drawType": "free1"}, false)
	send("/game/card/exp/use_items", map[string]any{"cardID": "1001", "items": map[int64]any{11: 3}}, true)
	send("/game/card/exp/use_items", map[string]any{"cardID": "1001", "items": map[int64]any{11: 1}}, false)
	send("/game/role/frag/comb", map[string]any{"fragID": 20011}, true)
	send("/game/role/main_task/gain", map[string]any{"taskID": 1000}, true)
	send("/game/role/main_task/gain", map[string]any{"taskID": 1000}, false)
	send("/game/battle/card", map[string]any{"battleCardIDs": []any{"1002", "1001"}}, true)
	send("/game/battle/card", map[string]any{"battleCardIDs": []any{"1002", "1002"}}, false)
	send("/game/ready/card/deploy", map[string]any{"idx": 1, "cardIDs": []any{"1003"}}, true)
	send("/game/card/locked/switch", map[string]any{"cardID": "1003"}, true)

	reopened, err := playerstore.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	save, err := reopened.Load(key)
	if err != nil {
		t.Fatal(err)
	}
	if len(save.Cards) != 3 || !save.Cards["1003"].Locked ||
		save.Role.RMB != 950 || save.Items[5000] != 10 || save.Items[11] != 0 ||
		save.Menu.Draw.FreeDiamond != 1 || !save.Menu.MainClaims[1000] ||
		save.Menu.Species[11] != 1 || len(save.BattleCards) != 2 ||
		len(save.Menu.Ready[1].Cards) != 1 || save.Menu.Ready[1].Cards[0] != "1003" {
		t.Fatalf("menu data did not survive reopening: %+v", save)
	}
}
