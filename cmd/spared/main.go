package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/spare-run/spare/internal/api"
	"github.com/spare-run/spare/internal/auth"
	"github.com/spare-run/spare/internal/dashboard"
	"github.com/spare-run/spare/internal/joblibrary"
	"github.com/spare-run/spare/internal/jobpackage"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/paths"
	"github.com/spare-run/spare/internal/preferences"
	"github.com/spare-run/spare/internal/profile"
	"github.com/spare-run/spare/internal/recipe"
	"github.com/spare-run/spare/internal/recipes"
	spareRuntime "github.com/spare-run/spare/internal/runtime"
	"github.com/spare-run/spare/internal/runtime/native"
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
	flags := flag.NewFlagSet("recipe-worker", flag.ContinueOnError)
	recipeID := flags.String("recipe", "", "recipe id")
	configValue := flags.String("config", "", "base64url-encoded recipe configuration")
	configStdin := flags.Bool("config-stdin", false, "read recipe configuration from standard input")
	port := flags.Int("port", 0, "recipe port")
	healthPort := flags.Int("health-port", 0, "health port")
	dataPath := flags.String("data-path", "", "private job data path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *recipeID == "" || (!*configStdin && *configValue == "") || *port == 0 || *healthPort == 0 {
		return errors.New("the recipe worker is missing required configuration")
	}
	var configJSON []byte
	var err error
	if *configStdin {
		configJSON, err = io.ReadAll(io.LimitReader(os.Stdin, 1024*1024+1))
		if err != nil || len(configJSON) > 1024*1024 {
			return errors.New("the recipe worker configuration is invalid")
		}
	} else {
		configJSON, err = base64.RawURLEncoding.DecodeString(*configValue)
		if err != nil {
			return errors.New("the recipe worker configuration is invalid")
		}
	}
	var values map[string]any
	if err := json.Unmarshal(configJSON, &values); err != nil {
		return errors.New("the recipe worker configuration is invalid")
	}
	registry, err := recipes.Trusted()
	if err != nil {
		return err
	}
	implementation, ok := registry.Get(*recipeID)
	if !ok {
		return fmt.Errorf("recipe %q is not built into this Spare release", *recipeID)
	}
	resolved, err := implementation.ResolveConfig(values)
	if err != nil {
		return err
	}
	if stateful, ok := implementation.(recipe.StatefulImplementation); ok {
		return stateful.ServeState(resolved, *port, *healthPort, *dataPath)
	}
	return implementation.Serve(resolved, *port, *healthPort)
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
	store, recovered, err := state.OpenRecovering(statePaths.Database)
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
	if recovered != nil {
		if err := store.AddEvent(context.Background(), model.Event{
			Level:   "warning",
			Kind:    "state_recovered",
			Message: "Spare recovered after its local database became unreadable.",
			Details: map[string]any{"preservedDatabase": filepath.Base(recovered.DatabasePath)},
		}); err != nil {
			return err
		}
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	registry, err := recipes.Trusted()
	if err != nil {
		return err
	}
	verifier, err := jobpackage.DefaultVerifier()
	if err != nil {
		return err
	}
	library, err := joblibrary.New(store, statePaths.JobPackages, version, registry, verifier)
	if err != nil {
		return err
	}
	runtimes := map[string]spareRuntime.Runtime{
		"native": &native.Driver{Executable: executable},
	}
	manager, err := supervisor.New(store, statePaths.Logs, machine, registry, runtimes)
	if err != nil {
		return err
	}
	manager.SetRecipeAvailability(library.Available)
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

	handler := api.NewServer(token, store, manager, dashboard.Files()).WithJobLibrary(library)
	server := &http.Server{
		Handler:           handler.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if preferences.Load(statePaths.Root).KeepRecipesRunningAfterLogin {
		manager.Restore()
	}
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
