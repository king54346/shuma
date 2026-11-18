package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shuma-server/internal/config"
	"shuma-server/internal/httpservice"
	"shuma-server/internal/login"
	"shuma-server/internal/protocol"
	"shuma-server/internal/tcpserver"
)

func TestMainHTTPEndpoints(t *testing.T) {
	cfg := config.Default()
	srv := httptest.NewServer(httpservice.NewMainHTTPMux(cfg))
	defer srv.Close()

	tests := []struct {
		path string
		key  string
	}{
		{"/version", "app_version"},
		{"/servers", "servers"},
		{"/notice", "notice"},
		{"/sdk/login", "token"},
	}

	for _, tc := range tests {
		resp, err := http.Get(srv.URL + tc.path)
		if err != nil {
			t.Fatalf("GET %s failed: %v", tc.path, err)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			resp.Body.Close()
			t.Fatalf("decode %s failed: %v", tc.path, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", tc.path, resp.StatusCode)
		}
		if _, ok := body[tc.key]; !ok {
			t.Fatalf("GET %s missing key %q in %v", tc.path, tc.key, body)
		}
	}
}

func TestReportAndWordCheckEndpoints(t *testing.T) {
	reportSrv := httptest.NewServer(httpservice.NewReportHTTPMux())
	defer reportSrv.Close()

	resp, err := http.Post(reportSrv.URL+"/feedback", "application/json", strings.NewReader(`{"msg":"test"}`))
	if err != nil {
		t.Fatalf("POST /feedback failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /feedback status = %d, want 200", resp.StatusCode)
	}

	wcSrv := httptest.NewServer(httpservice.NewWordCheckHTTPMux())
	defer wcSrv.Close()

	wcResp, err := http.Get(wcSrv.URL + "/check?word=hello")
	if err != nil {
		t.Fatalf("GET /check failed: %v", err)
	}
	var body map[string]any
	if err := json.NewDecoder(wcResp.Body).Decode(&body); err != nil {
		wcResp.Body.Close()
		t.Fatalf("decode /check failed: %v", err)
	}
	wcResp.Body.Close()
	if valid, ok := body["valid"].(bool); !ok || !valid {
		t.Fatalf("GET /check valid = %v, want true", body["valid"])
	}
}

func TestCapturedLoginCheckPacketAndTCPAck(t *testing.T) {
	capturedHex := "c800b00fc9fd16ebe2b700010c93569071b9c9cfd5faac7756dadbf2dad9cdc1f56042bc4db38e44c909e9f44c76782cf2af73751df44fc245d61952d8dbe4b937d2c848a61a148ae6d3d1b4cb6fc450510d0fd1d1d632af74cfc50bd56534e6b339a2c9566a0e9928209ca544a6b98d19fbaf7f5157fdd59570bea3b8aa49b5f6d2803e25a04f82e87851b7a9325e4e062800db3f858591cd9b9d6b9913fc129227a7d9312d130fef1ab1a698bb32b671f3f4ffe5c47153819079e1ec216bc5bc4c14837767393e24b44ff5de"
	rawPkt, err := hex.DecodeString(capturedHex)
	if err != nil {
		t.Fatalf("decode hex failed: %v", err)
	}

	// Verify standalone decode of the captured /login/check packet
	pkt, err := protocol.ReadPacket(bytes.NewReader(rawPkt), protocol.InitialKey())
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}
	if pkt.URL != "/login/check" || pkt.SynID != 1 {
		t.Fatalf("got url=%q synID=%d, want /login/check synID=1", pkt.URL, pkt.SynID)
	}
	m, ok := pkt.Data.(map[string]any)
	if !ok || m["name"] != "shuma_100001" {
		t.Fatalf("unexpected decoded payload: %#v", pkt.Data)
	}

	// Verify live TCP server responds with valid encrypted ACK
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- tcpserver.Serve(ctx, ln, "test-tcp", func(route string, req any) map[string]any {
			return login.Handle(config.Default(), route, req)
		})
	}()

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		cancel()
		t.Fatalf("dial failed: %v", err)
	}
	if _, err := conn.Write(rawPkt); err != nil {
		conn.Close()
		cancel()
		t.Fatalf("write failed: %v", err)
	}
	ackPkt, err := protocol.ReadPacket(conn, protocol.InitialKey())
	conn.Close()
	cancel()
	<-done

	if err != nil {
		t.Fatalf("ReadPacket ACK failed: %v", err)
	}
	if ackPkt.URL != "/login/check" || ackPkt.SynID != 1 {
		t.Fatalf("got ack url=%q synID=%d", ackPkt.URL, ackPkt.SynID)
	}
	ackMap, ok := ackPkt.Data.(map[string]any)
	if !ok || ackMap["ret"] != true {
		t.Fatalf("unexpected ack payload: %#v", ackPkt.Data)
	}
}
