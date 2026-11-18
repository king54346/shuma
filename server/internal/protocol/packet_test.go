package protocol

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

type chunkedReader struct {
	data  []byte
	pos   int
	chunk int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := r.chunk
	if n > len(p) {
		n = len(p)
	}
	if r.pos+n > len(r.data) {
		n = len(r.data) - r.pos
	}
	copy(p[:n], r.data[r.pos:r.pos+n])
	r.pos += n
	return n, nil
}

func TestControlFramesAndStickyPackets(t *testing.T) {
	key := InitialKey()

	f1, err := PackAckPacket("/login/check", 1, map[string]any{"ret": true, "seq": 1}, key)
	if err != nil {
		t.Fatalf("PackAckPacket f1: %v", err)
	}
	hb := PackHeartbeatPacket()
	okAck := PackOkAckPacket(42)
	errAck := PackErrAckPacket(43)
	f2, err := PackAckPacket("/game/sync", 2, map[string]any{"ret": true, "seq": 2}, key)
	if err != nil {
		t.Fatalf("PackAckPacket f2: %v", err)
	}

	// Concatenate all 5 frames into one stream (sticky packets).
	var stream bytes.Buffer
	stream.Write(f1)
	stream.Write(hb)
	stream.Write(okAck)
	stream.Write(errAck)
	stream.Write(f2)

	// Read with 1-byte fragmentation to test both sticky and fragmented parsing.
	r := &chunkedReader{data: stream.Bytes(), chunk: 1}

	p1, err := ReadPacket(r, key)
	if err != nil {
		t.Fatalf("ReadPacket p1: %v", err)
	}
	if p1.URL != "/login/check" || p1.SynID != 1 {
		t.Fatalf("p1 mismatch: url=%q synID=%d", p1.URL, p1.SynID)
	}

	pHB, err := ReadPacket(r, key)
	if err != nil {
		t.Fatalf("ReadPacket pHB: %v", err)
	}
	if !pHB.IsHeartbeat() || pHB.Flag != (FlagMagic|FlagHeartSyn) {
		t.Fatalf("pHB mismatch: %+v", pHB)
	}

	pOk, err := ReadPacket(r, key)
	if err != nil {
		t.Fatalf("ReadPacket pOk: %v", err)
	}
	if !pOk.IsControlACK() || !pOk.IsOkAck() || pOk.SynID != 42 {
		t.Fatalf("pOk mismatch: %+v", pOk)
	}

	pErr, err := ReadPacket(r, key)
	if err != nil {
		t.Fatalf("ReadPacket pErr: %v", err)
	}
	if !pErr.IsControlACK() || !pErr.IsErrAck() || pErr.SynID != 43 {
		t.Fatalf("pErr mismatch: %+v", pErr)
	}

	p2, err := ReadPacket(r, key)
	if err != nil {
		t.Fatalf("ReadPacket p2: %v", err)
	}
	if p2.URL != "/game/sync" || p2.SynID != 2 {
		t.Fatalf("p2 mismatch: url=%q synID=%d", p2.URL, p2.SynID)
	}

	if _, err := ReadPacket(r, key); err != io.EOF {
		t.Fatalf("expected EOF at end of stream, got %v", err)
	}
}

func TestLongLenPacketRoundTrip(t *testing.T) {
	key := InitialKey()
	largeText := strings.Repeat("shuma-long-payload-", 4000) // > 65535 bytes (literal-only LZ4 keeps > 65535)
	raw, err := PackAckPacket("/game/login", 99, map[string]any{"ret": true, "blob": largeText}, key)
	if err != nil {
		t.Fatalf("PackAckPacket long: %v", err)
	}
	if raw[0]&FlagLongLen == 0 {
		t.Fatalf("expected FlagLongLen set on 0x%02x", raw[0])
	}

	pkt, err := ReadPacket(&chunkedReader{data: raw, chunk: 7}, key)
	if err != nil {
		t.Fatalf("ReadPacket long: %v", err)
	}
	if pkt.URL != "/game/login" || pkt.SynID != 99 {
		t.Fatalf("pkt header mismatch: %+v", pkt)
	}
	m, ok := pkt.Data.(map[string]any)
	if !ok || m["blob"] != largeText {
		t.Fatalf("long payload mismatch")
	}
}

func TestInvalidMagicAndCRC(t *testing.T) {
	key := InitialKey()
	if _, err := ReadPacket(bytes.NewReader([]byte{0x01}), key); err == nil {
		t.Fatalf("expected invalid magic error")
	}

	pktBytes, err := PackAckPacket("/login/check", 1, map[string]any{"ret": true}, key)
	if err != nil {
		t.Fatalf("PackAckPacket: %v", err)
	}
	// Corrupt last byte (payload) to trigger CRC mismatch.
	pktBytes[len(pktBytes)-1] ^= 0xff
	if _, err := ReadPacket(bytes.NewReader(pktBytes), key); err == nil {
		t.Fatalf("expected crc32 mismatch error")
	}
}
