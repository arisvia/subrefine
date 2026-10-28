package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/arisvia/subrefine/internal/api"
	"github.com/arisvia/subrefine/internal/config"
	"github.com/arisvia/subrefine/internal/core/fetcher"
	"github.com/arisvia/subrefine/internal/service"
	"github.com/arisvia/subrefine/internal/storage"
)

// tuneRuntime trades a little CPU for a smaller resident set: with the GC
// defaults the heap is allowed to sit at twice the live set, which puts this
// service above its 15MB RSS budget on Windows. Explicit GOGC/GOMEMLIMIT in
// the environment win, so operators can still override without a rebuild.
func tuneRuntime() {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(50)
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(12 << 20)
	}
}

func main() {
	tuneRuntime()
	hostFlag := flag.String("host", "", "Server listen host (overrides config)")
	portFlag := flag.Int("port", 0, "Server listen port (overrides config)")
	dataDirFlag := flag.String("data", "", "Data directory (overrides config)")
	verFlag := flag.Bool("v", false, "Print version and exit")
	flag.Parse()

	if *verFlag {
		fmt.Printf("subrefine %s\n", api.Version)
		os.Exit(0)
	}

	cfg := config.DefaultConfig()
	config.LoadFromEnv(cfg)

	if *hostFlag != "" {
		cfg.Host = *hostFlag
	}
	if *portFlag > 0 {
		cfg.Port = *portFlag
	}
	if *dataDirFlag != "" {
		cfg.DataDir = *dataDirFlag
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("refusing to start: %v", err)
	}

	store, err := storage.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("failed to init storage: %v", err)
	}

	f := fetcher.New(15 * time.Second)
	subMgr := service.NewManager(store, f, 5*time.Minute)
	server := api.NewServer(cfg, subMgr, store)

	httpServer := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[subrefine %s] listening on %s (data: %s)", api.Version, cfg.Addr(), cfg.DataDir)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-stop
	log.Println("shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	log.Println("server stopped")
}
