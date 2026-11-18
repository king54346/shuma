package tcpserver

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"shuma-server/internal/protocol"
)

// Options configures heartbeat keepalive and idle connection reclamation.
type Options struct {
	// HeartbeatInterval controls how often the server sends a heart_syn (0x84)
	// frame when a connection has been idle. Zero disables proactive server heartbeats.
	HeartbeatInterval time.Duration
	// IdleTimeout closes connections that have had no read/write activity for this duration.
	// Zero disables idle eviction.
	IdleTimeout time.Duration
}

// DefaultOptions returns production defaults compatible with the client's 60s SockCheckTimeout.
func DefaultOptions() Options {
	return Options{
		HeartbeatInterval: 30 * time.Second,
		IdleTimeout:       180 * time.Second,
	}
}

// Serve accepts connections on ln and dispatches WANetTask requests to handle using DefaultOptions.
func Serve(ctx context.Context, ln net.Listener, label string, handle func(string, any) map[string]any) error {
	return ServeWithOptions(ctx, ln, label, DefaultOptions(), handle)
}

// ServeWithOptions accepts connections on ln with custom heartbeat and idle timeout settings.
func ServeWithOptions(ctx context.Context, ln net.Listener, label string, opts Options, handle func(string, any) map[string]any) error {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			serveConn(ctx, c, label, opts, handle)
		}(conn)
	}
}

func serveConn(ctx context.Context, c net.Conn, label string, opts Options, handle func(string, any) map[string]any) {
	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()
	defer c.Close()

	stop := context.AfterFunc(connCtx, func() { _ = c.Close() })
	defer stop()

	peer := c.RemoteAddr().String()
	log.Printf("[%s] Client connected: %s", label, peer)
	sessionKey := protocol.InitialKey()

	var writeMu sync.Mutex
	var lastRead atomic.Int64
	lastRead.Store(time.Now().UnixNano())

	writeFrame := func(b []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		if opts.IdleTimeout > 0 {
			_ = c.SetWriteDeadline(time.Now().Add(opts.IdleTimeout))
		}
		_, err := c.Write(b)
		return err
	}

	// Start idle/heartbeat monitor when either interval or timeout is configured.
	if opts.HeartbeatInterval > 0 || opts.IdleTimeout > 0 {
		tickInterval := opts.HeartbeatInterval
		if tickInterval <= 0 || (opts.IdleTimeout > 0 && opts.IdleTimeout/2 < tickInterval) {
			tickInterval = opts.IdleTimeout / 2
		}
		if tickInterval < 10*time.Millisecond {
			tickInterval = 10 * time.Millisecond
		}

		var hbWg sync.WaitGroup
		hbWg.Add(1)
		go func() {
			defer hbWg.Done()
			ticker := time.NewTicker(tickInterval)
			defer ticker.Stop()
			for {
				select {
				case <-connCtx.Done():
					return
				case now := <-ticker.C:
					idleFor := now.Sub(time.Unix(0, lastRead.Load()))
					if opts.IdleTimeout > 0 && idleFor >= opts.IdleTimeout {
						log.Printf("[%s] Closing idle connection %s (idle=%s)", label, peer, idleFor.Round(time.Millisecond))
						_ = c.Close()
						return
					}
				}
			}
		}()
		defer hbWg.Wait()
	}

	for {
		if opts.IdleTimeout > 0 {
			_ = c.SetReadDeadline(time.Now().Add(opts.IdleTimeout))
		}
		pkt, err := protocol.ReadPacket(c, sessionKey)
		if err != nil {
			if !errors.Is(err, io.EOF) && connCtx.Err() == nil && !errors.Is(err, net.ErrClosed) {
				log.Printf("[%s] Read/decode error from %s: %v", label, peer, err)
			}
			break
		}
		lastRead.Store(time.Now().UnixNano())

		if pkt.IsHeartbeat() {
			// Do not echo 0x84 back: client NetManager:onRecvPacket formats packet.synID
			// with %d before checking isHeartSyn() and crashes if synID is nil.
			continue
		}
		if pkt.IsControlACK() {
			log.Printf("[%s] Recv ok/err ack synID=%d from %s", label, pkt.SynID, peer)
			continue
		}

		log.Printf("[%s] Recv %s (synID=%d, len=%d)", label, pkt.URL, pkt.SynID, pkt.RawLen)

		resp := handle(pkt.URL, pkt.Data)
		ackBytes, err := protocol.PackAckPacket(pkt.URL, pkt.SynID, resp, sessionKey)
		if err != nil {
			log.Printf("[%s] PackAck error for %s: %v", label, pkt.URL, err)
			break
		}
		if err := writeFrame(ackBytes); err != nil {
			log.Printf("[%s] Write error to %s: %v", label, peer, err)
			break
		}
		log.Printf("[%s] Sent ACK %s (ackID=%d, %d bytes)", label, pkt.URL, pkt.SynID, len(ackBytes))
	}

	connCancel()
	log.Printf("[%s] Client disconnected: %s", label, peer)
}
