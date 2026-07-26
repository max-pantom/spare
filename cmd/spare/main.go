package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spare-run/spare/internal/api"
	"github.com/spare-run/spare/internal/auth"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/paths"
	"github.com/spare-run/spare/internal/profile"
	"github.com/spare-run/spare/internal/service"
	"github.com/spare-run/spare/internal/state"
	"github.com/spf13/cobra"
)

var version = "dev"

type app struct {
	paths paths.Paths
	out   io.Writer
	err   io.Writer
}

func main() {
	statePaths, err := paths.Resolve()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	application := &app{paths: statePaths, out: os.Stdout, err: os.Stderr}
	command := application.rootCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, formatError(err))
		os.Exit(1)
	}
}

func (a *app) rootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:           "spare",
		Short:         "Give this computer one useful job.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	command.AddCommand(
		a.initCommand(),
		a.tryCommand(),
		a.installCommand(),
		a.statusCommand(),
		a.openCommand(),
		a.actionCommand("start"),
		a.actionCommand("stop"),
		a.logsCommand(),
		a.doctorCommand(),
		a.removeCommand(),
		a.uninstallCommand(),
	)
	return command
}

func (a *app) initCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Prepare this computer to run Spare",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if err := a.paths.Ensure(); err != nil {
				return err
			}
			if _, err := auth.EnsureToken(a.paths.Token); err != nil {
				return err
			}
			store, err := state.Open(a.paths.Database)
			if err != nil {
				return err
			}
			var existing *model.Machine
			if current, readErr := store.Machine(command.Context()); readErr == nil {
				existing = &current
			}
			machine, err := profile.Collect(existing, a.paths.Root)
			if err == nil {
				err = store.SaveMachine(command.Context(), machine)
			}
			_ = store.Close()
			if err != nil {
				return err
			}

			daemon, err := findDaemon()
			if err != nil {
				return err
			}
			if err := service.InstallAndStart(command.Context(), daemon, a.paths.Root); err != nil {
				return err
			}
			client, err := a.waitForDaemon(command.Context(), 10*time.Second)
			if err != nil {
				return err
			}
			initialized, err := client.Machine(command.Context())
			if err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Spare is ready.\n\nMachine\n%s\n\nSystem\n%s/%s\n%d CPU cores\n%s memory\n\nTry Site\nspare try site ./public\n",
				initialized.Hostname,
				initialized.OS,
				initialized.Architecture,
				initialized.LogicalCores,
				formatBytes(initialized.MemoryTotalBytes),
			)
			return nil
		},
	}
}

func (a *app) tryCommand() *cobra.Command {
	var portValue string
	command := &cobra.Command{
		Use:   "try site <directory>",
		Short: "Run Site until this terminal closes",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			if args[0] != model.RecipeSite {
				return errors.New("only the built-in Site recipe is available")
			}
			port, portMode, err := parsePort(portValue)
			if err != nil {
				return err
			}
			client, err := api.Discover(a.paths)
			if err != nil {
				return errors.New("Spare is not initialized. Run `spare init` first")
			}
			instance, err := client.Create(command.Context(), model.ModeTemporary, args[1], portMode, port)
			if err != nil {
				return err
			}
			printInstance(a.out, "Site is running temporarily.", instance)
			fmt.Fprintln(a.out, "\nPress Ctrl-C to stop.")

			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					_ = client.Remove(cleanup, instance.ID)
					fmt.Fprintln(a.out, "\nTemporary Site stopped.")
					return nil
				case <-ticker.C:
					if err := client.Heartbeat(ctx, instance.ID); err != nil {
						return err
					}
				}
			}
		},
	}
	command.Flags().StringVar(&portValue, "port", "auto", "site port: auto or 1-65535")
	return command
}

func (a *app) installCommand() *cobra.Command {
	var root string
	var portValue string
	command := &cobra.Command{
		Use:   "install site",
		Short: "Keep Site running after login",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if args[0] != model.RecipeSite {
				return errors.New("only the built-in Site recipe is available")
			}
			if root == "" {
				return errors.New("choose a folder with `--path <directory>`")
			}
			port, portMode, err := parsePort(portValue)
			if err != nil {
				return err
			}
			client, err := api.Discover(a.paths)
			if err != nil {
				return errors.New("Spare is not initialized. Run `spare init` first")
			}
			instance, err := client.Create(command.Context(), model.ModeInstalled, root, portMode, port)
			if err != nil {
				return err
			}
			printInstance(a.out, "Site is installed.", instance)
			fmt.Fprintln(a.out, "\nIt will start automatically after you log in.")
			return nil
		},
	}
	command.Flags().StringVar(&root, "path", "", "folder to serve")
	command.Flags().StringVar(&portValue, "port", "auto", "site port: auto or 1-65535")
	return command
}

func (a *app) statusCommand() *cobra.Command {
	var asJSON bool
	command := &cobra.Command{
		Use:   "status",
		Short: "Show what this computer is doing",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			client, err := api.Discover(a.paths)
			if err != nil {
				return errors.New("Spare is not running. Run `spare init` to start it")
			}
			machine, err := client.Machine(command.Context())
			if err != nil {
				return err
			}
			instances, err := client.Instances(command.Context())
			if err != nil {
				return err
			}
			if asJSON {
				return writePrettyJSON(a.out, map[string]any{"machine": machine, "instances": instances})
			}
			fmt.Fprintf(a.out, "Spare\n\nMachine\n%s\n\n", machine.Hostname)
			if len(instances) == 0 {
				fmt.Fprintln(a.out, "This computer is ready.\n\nNo role installed.\n\nTry one with:\nspare try site ./public")
				return nil
			}
			instance := instances[0]
			fmt.Fprintf(a.out, "This computer is a Site.\n\nStatus\n%s\n", sentenceCase(instance.Status))
			for _, url := range instance.URLs {
				fmt.Fprintln(a.out, url)
			}
			if instance.Problem != nil {
				fmt.Fprintf(a.out, "\nNeeds attention\n%s\n%s\n", instance.Problem.Summary, instance.Problem.Recovery)
			}
			return nil
		},
	}
	command.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return command
}

func (a *app) openCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "open [dashboard|site]",
		Short: "Open Spare or the current Site",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			target := "dashboard"
			if len(args) == 1 {
				target = args[0]
			}
			client, err := api.Discover(a.paths)
			if err != nil {
				return err
			}
			var url string
			switch target {
			case "dashboard":
				url, err = client.BrowserSession(command.Context())
			case "site":
				var instances []model.Instance
				instances, err = client.Instances(command.Context())
				if err == nil {
					if len(instances) == 0 {
						return errors.New("Site is not installed")
					}
					url = instances[0].URLs[0]
				}
			default:
				return errors.New("choose `dashboard` or `site`")
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(a.out, url)
			return openBrowser(url)
		},
	}
}

func (a *app) actionCommand(action string) *cobra.Command {
	return &cobra.Command{
		Use:   action + " site",
		Short: sentenceCase(action) + " the installed Site",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if args[0] != model.RecipeSite {
				return fmt.Errorf("only Site can be %s", actionVerb(action))
			}
			client, err := api.Discover(a.paths)
			if err != nil {
				return err
			}
			instance, err := client.InstanceAction(command.Context(), model.RecipeSite, action)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Site is %s.\n", instance.Status)
			return nil
		},
	}
}

func (a *app) logsCommand() *cobra.Command {
	var follow bool
	command := &cobra.Command{
		Use:   "logs site",
		Short: "Read Site logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if args[0] != model.RecipeSite {
				return errors.New("only Site logs are available")
			}
			path := filepath.Join(a.paths.Logs, model.RecipeSite+".log")
			if !follow {
				data, err := os.ReadFile(path)
				if errors.Is(err, os.ErrNotExist) {
					return errors.New("Site has not written any logs yet")
				}
				if err != nil {
					return err
				}
				_, err = a.out.Write(data)
				return err
			}
			return followFile(command.Context(), path, a.out)
		},
	}
	command.Flags().BoolVarP(&follow, "follow", "f", false, "follow new log output")
	return command
}

func (a *app) doctorCommand() *cobra.Command {
	var asJSON bool
	command := &cobra.Command{
		Use:   "doctor",
		Short: "Check Spare and explain problems",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			type check struct {
				Name    string `json:"name"`
				Status  string `json:"status"`
				Message string `json:"message"`
			}
			checks := []check{}
			client, err := api.Discover(a.paths)
			if err != nil {
				checks = append(checks, check{Name: "Daemon", Status: "failed", Message: "Spare is not running. Run `spare init`."})
			} else if err := client.Health(command.Context()); err != nil {
				checks = append(checks, check{Name: "Daemon", Status: "failed", Message: err.Error()})
			} else {
				checks = append(checks, check{Name: "Daemon", Status: "healthy", Message: "The local management service is reachable."})
				instances, readErr := client.Instances(command.Context())
				if readErr != nil {
					checks = append(checks, check{Name: "Site", Status: "failed", Message: readErr.Error()})
				} else if len(instances) == 0 {
					checks = append(checks, check{Name: "Site", Status: "ready", Message: "No Site is installed."})
				} else {
					instance := instances[0]
					if _, statErr := os.Stat(instance.RootPath); statErr != nil {
						checks = append(checks, check{Name: "Site folder", Status: "failed", Message: "The selected Site folder is unavailable."})
					} else {
						checks = append(checks, check{Name: "Site folder", Status: "healthy", Message: "The selected folder is readable."})
					}
					checks = append(checks, check{Name: "Site process", Status: instance.Status, Message: problemMessage(instance)})
				}
			}
			if asJSON {
				return writePrettyJSON(a.out, map[string]any{"checks": checks})
			}
			fmt.Fprintln(a.out, "Checking Spare...")
			fmt.Fprintln(a.out)
			for _, item := range checks {
				fmt.Fprintf(a.out, "%-16s %-10s %s\n", item.Name, item.Status, item.Message)
			}
			return nil
		},
	}
	command.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return command
}

func (a *app) removeCommand() *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "remove site",
		Short: "Remove Site without deleting its folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if args[0] != model.RecipeSite {
				return errors.New("only Site can be removed")
			}
			if !yes {
				ok, err := confirm("Remove Site? The served folder will stay unchanged. [y/N] ")
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(a.out, "Site was not removed.")
					return nil
				}
			}
			client, err := api.Discover(a.paths)
			if err != nil {
				return err
			}
			if err := client.Remove(command.Context(), model.RecipeSite); err != nil {
				return err
			}
			fmt.Fprintln(a.out, "Site was removed. Its folder was left unchanged.")
			return nil
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	return command
}

func (a *app) uninstallCommand() *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove Spare from this user account",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				ok, err := confirm("Uninstall Spare? Site source folders will stay unchanged. [y/N] ")
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(a.out, "Spare was not uninstalled.")
					return nil
				}
			}
			var endpoint paths.Endpoint
			if client, err := api.Discover(a.paths); err == nil {
				endpoint, _ = a.paths.ReadEndpoint()
				_ = client.Remove(command.Context(), model.RecipeSite)
			}
			if err := service.Uninstall(command.Context(), a.paths.Root); err != nil {
				return err
			}
			if os.Getenv("SPARE_NO_SERVICE") == "1" && endpoint.PID > 0 {
				if process, err := os.FindProcess(endpoint.PID); err == nil {
					_ = process.Kill()
				}
			}
			time.Sleep(200 * time.Millisecond)
			if err := os.RemoveAll(a.paths.Root); err != nil {
				return err
			}
			fmt.Fprintln(a.out, "Spare was removed. Site source folders were left unchanged.")
			return nil
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	return command
}

func (a *app) waitForDaemon(ctx context.Context, timeout time.Duration) (*api.Client, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		client, err := api.Discover(a.paths)
		if err == nil {
			checkCtx, cancel := context.WithTimeout(ctx, time.Second)
			err = client.Health(checkCtx)
			cancel()
			if err == nil {
				return client, nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return nil, errors.New("Spare did not start. Run `spare doctor` and check the daemon log")
}

func findDaemon() (string, error) {
	if configured := os.Getenv("SPARED_PATH"); configured != "" {
		return filepath.Abs(configured)
	}
	current, err := os.Executable()
	if err == nil {
		name := "spared"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		sibling := filepath.Join(filepath.Dir(current), name)
		if _, statErr := os.Stat(sibling); statErr == nil {
			return sibling, nil
		}
	}
	path, err := exec.LookPath("spared")
	if err != nil {
		return "", errors.New("could not find `spared`; install the Spare CLI and daemon together")
	}
	return filepath.Abs(path)
}

func parsePort(value string) (int, string, error) {
	if value == "" || value == "auto" {
		return 0, "auto", nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, "", errors.New("choose `--port auto` or a port between 1 and 65535")
	}
	return port, "fixed", nil
}

func printInstance(output io.Writer, heading string, instance model.Instance) {
	fmt.Fprintln(output, heading)
	fmt.Fprintln(output, "\nAvailable at")
	for _, url := range instance.URLs {
		fmt.Fprintln(output, url)
	}
	fmt.Fprintln(output, "\nNearby devices can open a LAN address while connected to the same network.")
}

func openBrowser(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("unable to open a browser; open this address manually: %s", url)
	}
	return nil
}

func confirm(prompt string) (bool, error) {
	fmt.Fprint(os.Stderr, prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes", nil
}

func followFile(ctx context.Context, path string, output io.Writer) error {
	var offset int64
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		file, err := os.Open(path)
		if err == nil {
			info, _ := file.Stat()
			if info != nil && info.Size() < offset {
				offset = 0
			}
			_, _ = file.Seek(offset, io.SeekStart)
			written, _ := io.Copy(output, file)
			offset += written
			_ = file.Close()
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func writePrettyJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func formatBytes(value uint64) string {
	const gib = 1024 * 1024 * 1024
	if value >= gib {
		return fmt.Sprintf("%.1fGB", float64(value)/gib)
	}
	const mib = 1024 * 1024
	return fmt.Sprintf("%.0fMB", float64(value)/mib)
}

func problemMessage(instance model.Instance) string {
	if instance.Problem != nil {
		return instance.Problem.Summary + " " + instance.Problem.Recovery
	}
	return "Site is " + instance.Status + "."
}

func sentenceCase(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func actionVerb(action string) string {
	if action == "stop" {
		return "stopped"
	}
	return "started"
}

func formatError(err error) string {
	var clientError *api.ClientError
	if errors.As(err, &clientError) {
		if clientError.API.Hint != "" {
			return clientError.API.Message + "\n" + clientError.API.Hint
		}
		return clientError.API.Message
	}
	return err.Error()
}
