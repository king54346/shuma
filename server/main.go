package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"shuma-server/internal/config"
	"shuma-server/internal/game"
	"shuma-server/internal/httpservice"
	"shuma-server/internal/login"
	"shuma-server/internal/playerstore"
	"shuma-server/internal/tcpserver"
)

func main() {
	bindHost := flag.String("bind", config.Default().BindHost, "local IP to bind")
	portOffset := flag.Int("port-offset", 0, "offset for local listener ports, leaving advertised ports unchanged")
	saveDir := flag.String("save-dir", "saves", "directory for persistent player saves")
	flag.Parse()
	cfg := config.Default()
	cfg.BindHost = *bindHost
	addr := func(port int) string {
		return net.JoinHostPort(cfg.BindHost, fmt.Sprint(port+*portOffset))
	}
	store, err := playerstore.NewStore(*saveDir)
	if err != nil {
		log.Fatalf("Failed to initialize player store: %v", err)
	}
	defer store.Close()
	accountStore, err := login.NewAccountStore(*saveDir)
	if err != nil {
		log.Fatalf("Failed to initialize account store: %v", err)
	}
	defer accountStore.Close()
	gameManager, err := game.NewManager(store, accountStore, playerstore.AccountKey{
		Account:   "shuma_100001",
		ServerKey: "game.cn.1",
	})
	if err != nil {
		log.Fatalf("Failed to load player save: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpServers := []*http.Server{
		{
			Addr:              addr(cfg.MainHTTPPort),
			Handler:           httpservice.NewMainHTTPMux(cfg),
			ReadHeaderTimeout: 10 * time.Second,
		},
		{
			Addr:              addr(cfg.ReportPort),
			Handler:           httpservice.NewReportHTTPMux(),
			ReadHeaderTimeout: 10 * time.Second,
		},
		{
			Addr:              addr(cfg.WordCheckPort),
			Handler:           httpservice.NewWordCheckHTTPMux(),
			ReadHeaderTimeout: 10 * time.Second,
		},
	}

	loginLn, err := net.Listen("tcp", addr(cfg.LoginTCPPort))
	if err != nil {
		log.Fatalf("Failed to listen TCP %s:%d: %v", cfg.BindHost, cfg.LoginTCPPort, err)
	}
	gameLn, err := net.Listen("tcp", addr(cfg.GameTCPPort))
	if err != nil {
		log.Fatalf("Failed to listen TCP %s:%d: %v", cfg.BindHost, cfg.GameTCPPort, err)
	}

	var wg sync.WaitGroup
	for _, srv := range httpServers {
		wg.Add(1)
		go func(s *http.Server) {
			defer wg.Done()
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("HTTP server %s error: %v", s.Addr, err)
			}
		}(srv)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := tcpserver.Serve(ctx, loginLn, "login", func(route string, req any) map[string]any {
			return login.HandleWithAccounts(cfg, accountStore, route, req)
		}); err != nil {
			log.Printf("Login TCP server error: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := tcpserver.Serve(ctx, gameLn, "game", gameManager.Handle); err != nil {
			log.Printf("Game TCP server error: %v", err)
		}
	}()

	log.Printf(
		"Shuma Go backend started on %s (HTTP: %d, %d, %d | TCP Login: %d | TCP Game: %d | Advertised IP: %s)",
		cfg.BindHost,
		cfg.MainHTTPPort+*portOffset,
		cfg.ReportPort+*portOffset,
		cfg.WordCheckPort+*portOffset,
		cfg.LoginTCPPort+*portOffset,
		cfg.GameTCPPort+*portOffset,
		cfg.AdvertiseHost,
	)

	<-ctx.Done()
	log.Println("Shutting down servers...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, srv := range httpServers {
		_ = srv.Shutdown(shutdownCtx)
	}
	wg.Wait()
}
