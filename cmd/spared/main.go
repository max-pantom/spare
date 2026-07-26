package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spare-run/spare/internal/api"
	"github.com/spare-run/spare/internal/auth"
	"github.com/spare-run/spare/internal/dashboard"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/paths"
	"github.com/spare-run/spare/internal/profile"
	"github.com/spare-run/spare/internal/site"
	"github.com/spare-run/spare/internal/state"
	"github.com/spare-run/spare/internal/supervisor"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "worker" {
		if err := runWorker(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := runDaemon(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runWorker(args []string) error {
	if len(args) == 0 || args[0] != "site" {
		return errors.New("unknown Spare worker")
	}
	flags := flag.NewFlagSet("site-worker", flag.ContinueOnError)
	root := flags.String("path", "", "site root")
	port := flags.Int("port", 0, "site port")
	healthPort := flags.Int("health-port", 0, "health port")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *root == "" || *port == 0 || *healthPort == 0 {
		return errors.New("the Site worker is missing required configuration")
	}
	return site.Serve(*root, *port, *healthPort)
}

func runDaemon() error {
	statePaths, err := paths.Resolve()
	if err != nil {
		return err
	}
	if err := statePaths.Ensure(); err != nil {
		return err
	}
	token, err := auth.EnsureToken(statePaths.Token)
	if err != nil {
		return err
	}
	if alreadyRunning(statePaths, token) {
		return nil
	}
	store, err := state.Open(statePaths.Database)
	if err != nil {
		return fmt.Errorf("open Spare state: %w", err)
	}
	defer store.Close()

	var existingMachine = storeMachine(store)
	machine, err := profile.Collect(existingMachine, statePaths.Root)
	if err != nil {
		return err
	}
	if err := store.SaveMachine(context.Background(), machine); err != nil {
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	manager, err := supervisor.New(store, statePaths.Logs, executable, machine)
	if err != nil {
		return err
	}
	defer manager.Shutdown()

	listener, port, err := controlListener()
	if err != nil {
		return err
	}
	defer listener.Close()
	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	if err := statePaths.WriteEndpoint(paths.Endpoint{
		URL:       baseURL,
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC(),
	}); err != nil {
		return err
	}
	defer os.Remove(statePaths.Endpoint)

	handler := api.NewServer(token, store, manager, dashboard.Files())
	server := &http.Server{
		Handler:           handler.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	manager.Restore()
	slog.Info("Spare daemon started", "version", version, "url", baseURL)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func controlListener() (net.Listener, int, error) {
	for port := 7331; port <= 7339; port++ {
		listener, err := net.Listen("tcp4", "127.0.0.1:"+strconv.Itoa(port))
		if err == nil {
			return listener, port, nil
		}
	}
	return nil, 0, errors.New("Spare could not find a free local control port between 7331 and 7339")
}

func storeMachine(store *state.Store) *model.Machine {
	machine, err := store.Machine(context.Background())
	if err != nil {
		return nil
	}
	return &machine
}

func alreadyRunning(statePaths paths.Paths, token string) bool {
	endpoint, err := statePaths.ReadEndpoint()
	if err != nil {
		return false
	}
	client := api.NewClient(endpoint.URL, token)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	return client.Health(ctx) == nil
}
