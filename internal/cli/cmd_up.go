package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/localcloud-dev/localcloud/internal/adapters/mailpit"
	pgadapter "github.com/localcloud-dev/localcloud/internal/adapters/postgres"
	redisadapter "github.com/localcloud-dev/localcloud/internal/adapters/redis"
	"github.com/localcloud-dev/localcloud/internal/agent"
	"github.com/localcloud-dev/localcloud/internal/config"
	"github.com/localcloud-dev/localcloud/internal/docker"
	"github.com/localcloud-dev/localcloud/internal/proxy"
	"github.com/localcloud-dev/localcloud/internal/redaction"
)

func cmdUp(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "localcloud.yml", "Config file path")
	noCompose := fs.Bool("no-compose", false, "Skip Docker Compose start")
	verbose := fs.Bool("verbose", false, "Verbose logging")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Load config
	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "localcloud: %s\n", err)
		return 1
	}
	if errs := cfg.Validate(); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(stderr, "localcloud: %v\n", e)
		}
		return 1
	}

	// Logger
	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.Project.DataDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "localcloud: cannot create data dir: %s\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Docker Compose
	if !*noCompose && len(cfg.Compose.Files) > 0 {
		workDir, _ := filepath.Abs(filepath.Dir(*configPath))
		dc := docker.NewController(cfg.Compose.Files, cfg.Compose.ProjectName, workDir, logger)

		fmt.Fprintln(stdout, "Starting Docker Compose services...")
		if err := dc.Up(ctx); err != nil {
			fmt.Fprintf(stderr, "localcloud: compose up failed: %s\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Docker Compose services started.")
	}

	// Start agent
	a := agent.New(cfg, version, logger)

	// Register adapters from config
	var proxies []*proxy.Proxy
	for name, svc := range cfg.Services {
		switch svc.Type {
		case "http":
			if svc.Capture.ProxyPort == 0 {
				continue
			}
			listenAddr := fmt.Sprintf("%s:%d", cfg.Agent.Bind, svc.Capture.ProxyPort)
			p, err := proxy.New(proxy.Config{
				ServiceName:       name,
				TargetBaseURL:     svc.BaseURL,
				ListenAddr:        listenAddr,
				RunID:             a.RunID(),
				CorrelationHeader: cfg.Agent.CorrelationHeader,
				RedactionPolicy:   redaction.DefaultPolicy(),
				BodyMaxBytes:      cfg.Redaction.BodyMaxBytes,
			}, logger)
			if err != nil {
				fmt.Fprintf(stderr, "localcloud: proxy %s: %s\n", name, err)
				return 1
			}
			proxies = append(proxies, p)
			logger.Info("registered proxy", "service", name, "listen", listenAddr, "target", svc.BaseURL)

		case "postgres":
			if svc.DSN == "" {
				continue
			}
			adapter := pgadapter.New(
				name, svc.DSN, a.RunID(),
				svc.Capture.Tables,
				svc.Capture.RedactColumns,
				svc.Capture.Schemas,
				logger,
			)
			a.RegisterAdapter(adapter)
			logger.Info("registered adapter", "type", "postgres", "service", name)

		case "redis":
			if svc.Addr == "" {
				continue
			}
			adapter := redisadapter.New(
				name, svc.Addr, a.RunID(),
				svc.Capture.Queues,
				svc.Capture.RedactJSONPaths,
				logger,
			)
			a.RegisterAdapter(adapter)
			logger.Info("registered adapter", "type", "redis", "service", name)

		case "mailpit":
			if svc.APIUrl == "" {
				continue
			}
			pollInterval := 2 * time.Second
			if svc.Capture.PollInterval != "" {
				if d, err := time.ParseDuration(svc.Capture.PollInterval); err == nil {
					pollInterval = d
				}
			}
			adapter := mailpit.New(
				name, svc.APIUrl, a.RunID(),
				pollInterval,
				svc.Capture.RedactBody,
				logger,
			)
			a.RegisterAdapter(adapter)
			logger.Info("registered adapter", "type", "mailpit", "service", name)
		}
	}

	fmt.Fprintf(stdout, "Starting LocalCloud agent (run: %s)...\n", a.RunID())
	if err := a.Start(ctx); err != nil {
		fmt.Fprintf(stderr, "localcloud: agent start failed: %s\n", err)
		return 1
	}

	// Start HTTP proxies (need agent's sink)
	for _, p := range proxies {
		if err := p.Start(ctx, a.Sink()); err != nil {
			fmt.Fprintf(stderr, "localcloud: proxy start failed: %s\n", err)
			return 1
		}
	}

	studioURL := fmt.Sprintf("http://%s:%d", cfg.Agent.Bind, cfg.Agent.StudioPort)
	fmt.Fprintf(stdout, "LocalCloud is running.\n")
	fmt.Fprintf(stdout, "  Agent:  %s:%d\n", cfg.Agent.Bind, cfg.Agent.Port)
	fmt.Fprintf(stdout, "  Studio: %s\n", studioURL)
	fmt.Fprintf(stdout, "Press Ctrl+C to stop.\n")

	// Wait for shutdown signal
	<-ctx.Done()
	fmt.Fprintln(stdout, "\nShutting down...")

	shutdownCtx := context.Background()

	// Stop proxies first
	for _, p := range proxies {
		if err := p.Stop(shutdownCtx); err != nil {
			logger.Error("proxy stop error", "err", err)
		}
	}

	if err := a.Stop(shutdownCtx); err != nil {
		fmt.Fprintf(stderr, "localcloud: agent stop error: %s\n", err)
	}

	fmt.Fprintln(stdout, "LocalCloud stopped.")
	return 0
}

func cmdDown(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "localcloud.yml", "Config file path")
	volumes := fs.Bool("volumes", false, "Also remove Docker volumes")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "localcloud: %s\n", err)
		return 1
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx := context.Background()

	if len(cfg.Compose.Files) > 0 {
		workDir, _ := filepath.Abs(filepath.Dir(*configPath))
		dc := docker.NewController(cfg.Compose.Files, cfg.Compose.ProjectName, workDir, logger)

		fmt.Fprintln(stdout, "Stopping Docker Compose services...")
		if err := dc.Down(ctx); err != nil {
			fmt.Fprintf(stderr, "localcloud: compose down failed: %s\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Docker Compose services stopped.")
	}

	_ = volumes // TODO: pass -v flag to compose down

	fmt.Fprintln(stdout, "LocalCloud stopped.")
	return 0
}

func cmdStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "localcloud.yml", "Config file path")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "localcloud: %s\n", err)
		return 1
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	ctx := context.Background()

	if len(cfg.Compose.Files) > 0 {
		workDir, _ := filepath.Abs(filepath.Dir(*configPath))
		dc := docker.NewController(cfg.Compose.Files, cfg.Compose.ProjectName, workDir, logger)
		ps, err := dc.PS(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "localcloud: compose ps failed: %s\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Docker Compose services:")
		fmt.Fprintln(stdout, ps)
	}

	return 0
}
