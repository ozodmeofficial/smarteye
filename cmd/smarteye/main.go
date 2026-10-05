// Command smarteye is the single SmartEYE executable. The same binary runs as
// either the control-side server (o'qituvchi/rahbar) or the client agent
// (o'quvchi/xodim); the role is chosen at install time and stored in config.
//
// Usage:
//
//	smarteye                      run using the stored role
//	smarteye --setup --role server --name "A xona" --code "482913"
//	smarteye --setup --role client --code "482913"
//	smarteye --role server        run as server for this launch (no persist)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ozodmeofficial/smarteye/internal/agent"
	"github.com/ozodmeofficial/smarteye/internal/config"
	"github.com/ozodmeofficial/smarteye/internal/meta"
	"github.com/ozodmeofficial/smarteye/internal/security"
	"github.com/ozodmeofficial/smarteye/internal/server"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	log.SetPrefix("SmartEYE ")
	setupLogging()

	var (
		role    = flag.String("role", "", "run as 'server' or 'client' (overrides stored role for this launch)")
		setup   = flag.Bool("setup", false, "write configuration from flags, then exit (used by the installer)")
		name    = flag.String("name", "", "server display name (server role)")
		code    = flag.String("code", "", "network pairing code shared across the fleet")
		host    = flag.String("server", "", "manual server host for a client (fallback to auto-discovery)")
		lang    = flag.String("lang", "", "UI language: uz, ru, en")
		noOpen  = flag.Bool("no-open", false, "do not open the dashboard browser window (server role)")
		showVer = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Printf("%s %s (protocol %s)\n", meta.Product, meta.Version, "1")
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Apply any configuration flags.
	changed := applyFlags(cfg, *role, *name, *code, *host, *lang)

	if *setup {
		// Installer path: generate a code for a fresh server if none provided.
		if cfg.Role == config.RoleServer && cfg.NetCode == "" {
			c, err := security.GenerateCode()
			if err != nil {
				log.Fatalf("generate code: %v", err)
			}
			cfg.NetCode = c
		}
		if err := cfg.Save(); err != nil {
			log.Fatalf("save config: %v", err)
		}
		fmt.Printf("Configured role=%s", cfg.Role)
		if cfg.Role == config.RoleServer {
			fmt.Printf(" code=%s", cfg.NetCode)
		}
		fmt.Println()
		return
	}
	if changed {
		_ = cfg.Save()
	}

	if !cfg.IsConfigured() {
		fmt.Fprintln(os.Stderr, "SmartEYE is not configured. Run with --setup --role server|client --code <CODE>.")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cfg.Role {
	case config.RoleServer:
		runServer(ctx, cfg, *noOpen)
	case config.RoleClient:
		runClient(ctx, cfg)
	default:
		log.Fatalf("unknown role %q", cfg.Role)
	}
}

func applyFlags(cfg *config.Config, role, name, code, host, lang string) bool {
	changed := false
	switch role {
	case "server":
		cfg.Role = config.RoleServer
		changed = true
	case "client":
		cfg.Role = config.RoleClient
		changed = true
	}
	if name != "" {
		cfg.ServerName = name
		changed = true
	}
	if code != "" {
		cfg.NetCode = config.NormalizeNetCode(code)
		changed = true
	}
	if host != "" {
		cfg.ServerHost = host
		changed = true
	}
	if lang != "" {
		cfg.Language = lang
		changed = true
	}
	if cfg.Role == config.RoleServer && cfg.ServerName == "" {
		h, _ := os.Hostname()
		cfg.ServerName = h
		changed = true
	}
	return changed
}

func runServer(ctx context.Context, cfg *config.Config, noOpen bool) {
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("server init: %v", err)
	}
	// Start, then open the dashboard once the UI address is known.
	go func() {
		// Give Run a moment to bind the UI listener.
		for i := 0; i < 50 && srv.UIAddr == ""; i++ {
			sleepMS(20)
		}
		log.Printf("Dashboard: %s   Network code: %s", srv.UIAddr, cfg.NetCode)
		if !noOpen && srv.UIAddr != "" {
			openDashboard(srv.UIAddr)
		}
	}()
	if err := srv.Run(ctx); err != nil && err != context.Canceled {
		log.Printf("server stopped: %v", err)
	}
}

func runClient(ctx context.Context, cfg *config.Config) {
	ag, err := agent.New(cfg)
	if err != nil {
		log.Fatalf("agent init: %v", err)
	}
	log.Printf("Client agent started (device %s). Searching for server...", cfg.DeviceID[:8])
	if err := ag.Run(ctx); err != nil && err != context.Canceled {
		log.Printf("agent stopped: %v", err)
	}
}
