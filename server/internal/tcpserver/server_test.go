package tcpserver

import (
	"context"
	"net"
	"testing"
	"time"

	"shuma-server/internal/protocol"
)

func TestHeartbeatEchoAndSubsequentBusinessRequest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ServeWithOptions(ctx, ln, "test-hb", Options{
			HeartbeatInterval: 50 * time.Millisecond,
			IdleTimeout:       500 * time.Millisecond,
		}, func(url string, req any) map[string]any {
			return map[string]any{"ret": true, "url": url}
		})
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	// Client sends heart_syn + ok_ack + business request in one write (sticky).
	key := protocol.InitialKey()
	reqFrame, err := protocol.PackAckPacket("/game/sync", 7, map[string]any{"ping": 1}, key)
	if err != nil {
		t.Fatalf("PackAckPacket: %v", err)
	}
	var sticky []byte
	sticky = append(sticky, protocol.PackHeartbeatPacket()...)
	sticky = append(sticky, protocol.PackOkAckPacket(1)...)
	sticky = append(sticky, reqFrame...)
	if _, err := conn.Write(sticky); err != nil {
		t.Fatalf("Write sticky: %v", err)
	}

	// Server consumes heart_syn and ok_ack silently and responds to /game/sync.
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	p, err := protocol.ReadPacket(conn, key)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	if p.URL != "/game/sync" || p.SynID != 7 {
		t.Fatalf("expected /game/sync synID=7, got %+v", p)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("ServeWithOptions exit: %v", err)
	}
}

func TestIdleConnectionReclaimed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = ServeWithOptions(ctx, ln, "test-idle", Options{
			HeartbeatInterval: 0, // disable server heartbeat so connection stays truly idle
			IdleTimeout:       80 * time.Millisecond,
		}, func(url string, req any) map[string]any {
			return map[string]any{"ret": true}
		})
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := protocol.ReadPacket(conn, protocol.InitialKey()); err == nil {
		t.Fatalf("expected idle connection to be closed by server")
	}
}
