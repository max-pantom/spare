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
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spare-run/spare/internal/api"
	"github.com/spare-run/spare/internal/backup"
	"github.com/spare-run/spare/internal/bootstrap"
	"github.com/spare-run/spare/internal/doctor"
	"github.com/spare-run/spare/internal/jobpackage"
	"github.com/spare-run/spare/internal/model"
	"github.com/spare-run/spare/internal/paths"
	"github.com/spare-run/spare/internal/permissions"
	"github.com/spare-run/spare/internal/profile"
	"github.com/spare-run/spare/internal/recipe"
	"github.com/spare-run/spare/internal/recipes"
	"github.com/spare-run/spare/internal/recipeview"
	"github.com/spare-run/spare/internal/support"
	"github.com/spare-run/spare/internal/uninstall"
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
		a.recipeCommand(),
		a.jobCommand(),
		a.viewCommand(),
		a.tryCommand(),
		a.installCommand(),
		a.statusCommand(),
		a.openCommand(),
		a.actionCommand("start"),
		a.actionCommand("stop"),
		a.logsCommand(),
		a.doctorCommand(),
		a.supportCommand(),
		a.removeCommand(),
		a.exportCommand(),
		a.importCommand(),
		a.uninstallCommand(),
	)
	return command
}

func (a *app) supportCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "support",
		Short: "Create privacy-safe diagnostics for a support request",
	}
	command.AddCommand(&cobra.Command{
		Use:   "bundle [destination.zip]",
		Short: "Create a support bundle without private content",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			destination := support.DefaultName(time.Now())
			if len(args) == 1 {
				destination = args[0]
			}
			client, err := api.Discover(a.paths)
			if err != nil {
				client = nil
			}
			created, err := support.Create(command.Context(), destination, version, client, a.paths)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Support bundle created:\n%s\n", created)
			return nil
		},
	})
	return command
}

func (a *app) viewCommand() *cobra.Command {
	var noOpen bool
	command := &cobra.Command{
		Use:   "view <recipe|package.sp>",
		Short: "Open a built-in recipe or package in a safe local viewer",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			source, err := a.recipePackageReference(args[0])
			if err != nil {
				return err
			}
			viewer, err := recipeview.New(source)
			if err != nil {
				return err
			}
			running, err := viewer.Start()
			if err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Viewing %s\n%s\n\nClose the browser tab when you finish. Press Ctrl-C to stop the viewer now.\n",
				viewer.Summary().FileName,
				running.URL,
			)
			if !noOpen {
				if err := openBrowser(running.URL); err != nil {
					_ = running.Close()
					return err
				}
			}
			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return running.Wait(ctx)
		},
	}
	command.Flags().BoolVar(&noOpen, "no-open", false, "print the viewer URL without opening a browser")
	return command
}

func (a *app) initCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Prepare this computer to run Spare",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			daemon, err := bootstrap.FindDaemon()
			if err != nil {
				return err
			}
			_, initialized, err := bootstrap.Ensure(command.Context(), a.paths, daemon)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Spare is ready.\n\nMachine\n%s\n\nSystem\n%s/%s\n%d CPU cores\n%s memory\n\nTry a recipe\nspare try site ./public\nspare try drop ./received-files\nspare try hook\n",
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

func (a *app) recipeCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "recipe",
		Short: "Inspect and package recipes",
	}
	command.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List built-in recipes",
			Args:  cobra.NoArgs,
			RunE: func(command *cobra.Command, args []string) error {
				registry, err := recipes.Builtins()
				if err != nil {
					return err
				}
				var machine model.Machine
				if client, discoverErr := api.Discover(a.paths); discoverErr == nil {
					machine, _ = client.Machine(command.Context())
				} else {
					machine, _ = profile.Collect(nil, ".")
				}
				for _, available := range registry.Models(machine) {
					fmt.Fprintf(a.out, "%s\t%s\t%s\n", available.ID, available.Title, available.Compatibility.Rating)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "validate <recipe|directory|manifest|package.sp>",
			Short: "Validate a recipe manifest or package",
			Args:  cobra.ExactArgs(1),
			RunE: func(command *cobra.Command, args []string) error {
				manifest, builtIn, err := a.recipeManifestReference(args[0])
				if err != nil {
					return err
				}
				compatibility := recipe.CurrentPlatformCompatible(manifest)
				kind := ""
				if builtIn {
					kind = " built-in"
				}
				fmt.Fprintf(a.out, "%s %s is a valid%s recipe.\nCompatibility: %s\n", manifest.Name, manifest.Version, kind, compatibility.Rating)
				if packagePath, packageErr := a.findBuiltInPackage(manifest); builtIn && packageErr == nil {
					fmt.Fprintf(a.out, "Package: %s\n", packagePath)
				}
				return nil
			},
		},
		a.recipePackCommand(),
		a.recipeSignCommand(),
		&cobra.Command{
			Use:   "inspect <recipe|directory|manifest|package.sp>",
			Short: "Print a recipe manifest",
			Args:  cobra.ExactArgs(1),
			RunE: func(command *cobra.Command, args []string) error {
				manifest, _, err := a.recipeManifestReference(args[0])
				if err != nil {
					return err
				}
				return writePrettyJSON(a.out, manifest)
			},
		},
	)
	return command
}

func (a *app) recipeSignCommand() *cobra.Command {
	var keyPath string
	var minimumSpare string
	command := &cobra.Command{
		Use:   "sign <package.sp>",
		Short: "Sign a first-party catalog package",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if keyPath == "" {
				keyPath = os.Getenv("SPARE_CATALOG_SIGNING_KEY")
			}
			if keyPath == "" {
				return errors.New("set SPARE_CATALOG_SIGNING_KEY or use --key")
			}
			key, err := jobpackage.LoadPrivateKey(keyPath)
			if err != nil {
				return err
			}
			envelope, err := jobpackage.Sign(args[0], key, minimumSpare)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Signed %s\nPublisher: %s\nDigest: %s\n", args[0], envelope.Publisher, envelope.Digest)
			return nil
		},
	}
	command.Flags().StringVar(&keyPath, "key", "", "Ed25519 private key in PKCS#8 PEM format")
	command.Flags().StringVar(&minimumSpare, "minimum-spare-version", "0.1.1-alpha.3", "oldest compatible Spare version")
	return command
}

func (a *app) jobCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "job",
		Short: "Manage optional first-party jobs",
	}
	command.AddCommand(
		&cobra.Command{
			Use:   "add <package.sp>",
			Short: "Verify and add a downloaded job package",
			Args:  cobra.ExactArgs(1),
			RunE: func(command *cobra.Command, args []string) error {
				client, err := api.Discover(a.paths)
				if err != nil {
					return err
				}
				review, err := client.ReviewJobPackage(command.Context(), args[0])
				if err != nil {
					return err
				}
				fmt.Fprintf(a.out, "%s %s\nPublisher: %s\nSignature: %s\n", review.Title, review.Version, review.Publisher, review.SignatureStatus)
				for _, permission := range review.Permissions {
					if permission.Granted {
						fmt.Fprintf(a.out, "- %s\n", permission.Description)
					}
				}
				value, err := client.InstallJobPackage(command.Context(), args[0])
				if err != nil {
					return err
				}
				fmt.Fprintf(a.out, "\nInstalled %s %s. The active job was not changed.\n", review.Title, value.Version)
				return nil
			},
		},
		&cobra.Command{
			Use:   "remove <job>",
			Short: "Uninstall an inactive optional job",
			Args:  cobra.ExactArgs(1),
			RunE: func(command *cobra.Command, args []string) error {
				client, err := api.Discover(a.paths)
				if err != nil {
					return err
				}
				if err := client.UninstallJobPackage(command.Context(), args[0]); err != nil {
					return err
				}
				fmt.Fprintf(a.out, "Uninstalled %s. Job data was left unchanged.\n", args[0])
				return nil
			},
		},
	)
	return command
}

func (a *app) recipeManifestReference(reference string) (recipe.Manifest, bool, error) {
	registry, err := recipes.Builtins()
	if err != nil {
		return recipe.Manifest{}, false, err
	}
	if implementation, ok := registry.Get(strings.ToLower(reference)); ok {
		manifest := implementation.Manifest()
		if packagePath, packageErr := a.findBuiltInPackage(manifest); packageErr == nil {
			packaged, loadErr := recipe.Load(packagePath)
			if loadErr != nil {
				return recipe.Manifest{}, true, loadErr
			}
			if !reflect.DeepEqual(packaged, manifest) {
				return recipe.Manifest{}, true, fmt.Errorf(
					"bundled package %s does not match the trusted built-in %s manifest",
					packagePath,
					manifest.Name,
				)
			}
		}
		return manifest, true, nil
	}
	manifest, err := recipe.Load(reference)
	return manifest, false, err
}

func (a *app) recipePackageReference(reference string) (string, error) {
	if info, err := os.Stat(reference); err == nil {
		if info.Mode().IsRegular() && strings.EqualFold(filepath.Ext(reference), ".sp") {
			return filepath.Abs(reference)
		}
		return "", fmt.Errorf("%s is not a .sp recipe package", reference)
	}
	registry, err := recipes.Builtins()
	if err != nil {
		return "", err
	}
	implementation, ok := registry.Get(strings.ToLower(reference))
	if !ok {
		return "", fmt.Errorf(
			"%q is neither a built-in recipe ID nor a readable .sp package; use `spare recipe list` to see the defaults",
			reference,
		)
	}
	return a.findBuiltInPackage(implementation.Manifest())
}

func (a *app) findBuiltInPackage(manifest recipe.Manifest) (string, error) {
	name := fmt.Sprintf("%s_%s.sp", manifest.ID, manifest.Version)
	candidates := []string{
		filepath.Join(a.paths.Root, "recipes", name),
	}
	if executable, err := os.Executable(); err == nil {
		if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
			executable = resolved
		}
		directory := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(directory, "recipes", name),
			filepath.Join(directory, "..", "recipes", name),
		)
	}
	if workingDirectory, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(workingDirectory, "recipes", name),
			filepath.Join(workingDirectory, "dist", "recipes", name),
			filepath.Join(workingDirectory, "dist", "releases", name),
		)
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil || seen[absolute] {
			continue
		}
		seen[absolute] = true
		info, err := os.Stat(absolute)
		if err == nil && info.Mode().IsRegular() {
			return absolute, nil
		}
	}
	return "", fmt.Errorf(
		"%s is built into Spare, but its viewable package %s was not found; reinstall Spare with its bundled recipes",
		manifest.Name,
		name,
	)
}

func (a *app) recipePackCommand() *cobra.Command {
	var output string
	command := &cobra.Command{
		Use:   "pack <directory>",
		Short: "Create a checksummable .sp recipe package",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			manifest, err := recipe.Load(args[0])
			if err != nil {
				return err
			}
			if output == "" {
				output = manifest.ID + ".sp"
			}
			absolute, err := filepath.Abs(output)
			if err != nil {
				return err
			}
			manifest, err = recipe.Pack(args[0], absolute)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Created %s\nRecipe: %s %s\n", absolute, manifest.Name, manifest.Version)
			return nil
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "", "output .sp path")
	return command
}

func (a *app) tryCommand() *cobra.Command {
	var pathValue string
	var portValue string
	var maximumFileSize string
	command := &cobra.Command{
		Use:   "try <recipe|package.sp> [directory]",
		Short: "Run a recipe until this terminal closes",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(command *cobra.Command, args []string) error {
			manifest, err := a.runnableManifest(args[0])
			if err != nil {
				return err
			}
			if len(args) == 2 {
				if pathValue != "" {
					return errors.New("choose the recipe folder with either the positional argument or `--path`, not both")
				}
				pathValue = args[1]
			}
			values, err := commandConfig(manifest, pathValue, maximumFileSize)
			if err != nil {
				return err
			}
			port, portMode, err := parsePort(portValue)
			if err != nil {
				return err
			}
			client, err := api.Discover(a.paths)
			if err != nil {
				return errors.New("Spare is not initialized. Run `spare init` first")
			}
			current, err := client.Create(command.Context(), manifest.ID, model.ModeTemporary, values, portMode, port)
			if err != nil {
				return err
			}
			printInstance(a.out, manifest.Name+" is running temporarily.", manifest.Name, current)
			fmt.Fprintln(a.out, "\nPress Ctrl-C to stop.")

			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					_ = client.Remove(cleanup, current.ID)
					cancel()
					fmt.Fprintf(a.out, "\nTemporary %s stopped.\n", manifest.Name)
					return nil
				case <-ticker.C:
					if err := client.Heartbeat(ctx, current.ID); err != nil {
						return err
					}
				}
			}
		},
	}
	command.Flags().StringVar(&pathValue, "path", "", "selected folder")
	command.Flags().StringVar(&portValue, "port", "auto", "recipe port: auto or 1-65535")
	command.Flags().StringVar(&maximumFileSize, "max-file-size", "", "Drop maximum file size, such as 2GB")
	return command
}

func (a *app) installCommand() *cobra.Command {
	var pathValue string
	var portValue string
	var maximumFileSize string
	command := &cobra.Command{
		Use:   "install <recipe|package.sp>",
		Short: "Keep a recipe running after login",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			manifest, err := a.runnableManifest(args[0])
			if err != nil {
				return err
			}
			values, err := commandConfig(manifest, pathValue, maximumFileSize)
			if err != nil {
				return err
			}
			port, portMode, err := parsePort(portValue)
			if err != nil {
				return err
			}
			client, err := api.Discover(a.paths)
			if err != nil {
				return errors.New("Spare is not initialized. Run `spare init` first")
			}
			current, err := client.Create(command.Context(), manifest.ID, model.ModeInstalled, values, portMode, port)
			if err != nil {
				return err
			}
			printPermissions(a.out, manifest)
			printInstance(a.out, manifest.Name+" is installed.", manifest.Name, current)
			fmt.Fprintln(a.out, "\nIt will start automatically after you log in.")
			return nil
		},
	}
	command.Flags().StringVar(&pathValue, "path", "", "selected folder")
	command.Flags().StringVar(&portValue, "port", "auto", "recipe port: auto or 1-65535")
	command.Flags().StringVar(&maximumFileSize, "max-file-size", "", "Drop maximum file size, such as 2GB")
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
			available, err := client.Recipes(command.Context())
			if err != nil {
				return err
			}
			if asJSON {
				return writePrettyJSON(a.out, map[string]any{
					"machine":   machine,
					"recipes":   available,
					"instances": instances,
				})
			}
			fmt.Fprintf(a.out, "Spare\n\nMachine\n%s\n\n", machine.Hostname)
			if len(instances) == 0 {
				fmt.Fprintln(a.out, "This computer is ready.\n\nNo role installed.\n\nTry one with:\nspare try site ./public\nspare try drop ./received-files\nspare try hook")
				return nil
			}
			current := instances[0]
			title := titleForRecipe(available, current.RecipeID)
			fmt.Fprintf(a.out, "This computer is a %s.\n\nStatus\n%s\n", title, sentenceCase(current.Status))
			for _, url := range current.URLs {
				fmt.Fprintln(a.out, url)
			}
			if current.ItemCount > 0 {
				fmt.Fprintf(a.out, "\nFiles\n%d\n", current.ItemCount)
			}
			if current.StorageAvailableBytes > 0 {
				fmt.Fprintf(a.out, "\nAvailable storage\n%s\n", formatBytes(current.StorageAvailableBytes))
			}
			if current.Problem != nil {
				fmt.Fprintf(a.out, "\nNeeds attention\n%s\n%s\n", current.Problem.Summary, current.Problem.Recovery)
			}
			return nil
		},
	}
	command.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return command
}

func (a *app) openCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "open [dashboard|recipe]",
		Short: "Open Spare or the current recipe",
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
			if target == "dashboard" {
				url, err = client.BrowserSession(command.Context())
			} else {
				var instances []model.Instance
				instances, err = client.Instances(command.Context())
				if err == nil {
					if len(instances) == 0 {
						return errors.New("no recipe is installed")
					}
					if target != "recipe" && target != instances[0].RecipeID {
						return fmt.Errorf("%s is not installed", target)
					}
					url = instances[0].URLs[0]
				}
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
		Use:   action + " <recipe>",
		Short: sentenceCase(action) + " the installed recipe",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := api.Discover(a.paths)
			if err != nil {
				return err
			}
			current, err := client.InstanceAction(command.Context(), args[0], action)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.out, "%s is %s.\n", sentenceCase(current.RecipeID), current.Status)
			return nil
		},
	}
}

func (a *app) logsCommand() *cobra.Command {
	var follow bool
	command := &cobra.Command{
		Use:   "logs <recipe>",
		Short: "Read recipe logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			manifest, err := a.runnableManifest(args[0])
			if err != nil {
				return err
			}
			path := filepath.Join(a.paths.Logs, manifest.ID+".log")
			if !follow {
				data, err := os.ReadFile(path)
				if errors.Is(err, os.ErrNotExist) {
					return errors.New("this recipe has not written any logs yet")
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
	var security bool
	command := &cobra.Command{
		Use:   "doctor",
		Short: "Check Spare and explain problems",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			client, err := api.Discover(a.paths)
			if security {
				if err != nil {
					client = nil
				}
				report := doctor.RunSecurity(command.Context(), client, a.paths)
				if asJSON {
					return writePrettyJSON(a.out, report)
				}
				printDoctor(a.out, report)
				return nil
			}
			if err != nil {
				report := doctor.Run(command.Context(), nil, a.paths)
				if asJSON {
					return writePrettyJSON(a.out, report)
				}
				printDoctor(a.out, report)
				return nil
			}
			report := doctor.Run(command.Context(), client, a.paths)
			if asJSON {
				return writePrettyJSON(a.out, report)
			}
			printDoctor(a.out, report)
			return nil
		},
	}
	command.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	command.Flags().BoolVar(&security, "security", false, "check local security boundaries and exposure")
	return command
}

func (a *app) removeCommand() *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "remove <recipe>",
		Short: "Remove a recipe without deleting its selected folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			manifest, err := a.runnableManifest(args[0])
			if err != nil {
				return err
			}
			if !yes {
				ok, err := confirm("Remove " + manifest.Name + "? Its selected folder will stay unchanged. [y/N] ")
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintf(a.out, "%s was not removed.\n", manifest.Name)
					return nil
				}
			}
			client, err := api.Discover(a.paths)
			if err != nil {
				return err
			}
			if err := client.Remove(command.Context(), manifest.ID); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "%s was removed. Its selected folder was left unchanged.\n", manifest.Name)
			return nil
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	return command
}

func (a *app) exportCommand() *cobra.Command {
	var output string
	command := &cobra.Command{
		Use:   "export <recipe>",
		Short: "Export recipe configuration and selected-folder data",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := api.Discover(a.paths)
			if err != nil {
				return err
			}
			current, err := client.Instance(command.Context(), args[0])
			if err != nil {
				return err
			}
			if output == "" {
				output = fmt.Sprintf("%s-%s.spare-backup", current.RecipeID, time.Now().Format("20060102-150405"))
			}
			absolute, err := filepath.Abs(output)
			if err != nil {
				return err
			}
			if err := backup.ExportInstance(current, absolute); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Created %s\nThe selected folder was copied into this backup.\n", absolute)
			return nil
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "", "backup output path")
	return command
}

func (a *app) importCommand() *cobra.Command {
	var destination string
	var portValue string
	command := &cobra.Command{
		Use:   "import <backup.spare-backup>",
		Short: "Restore recipe data and install its configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if destination == "" {
				return errors.New("choose an empty destination with `--path <directory>`")
			}
			absoluteDestination, err := filepath.Abs(destination)
			if err != nil {
				return err
			}
			manifest, err := backup.Import(args[0], absoluteDestination)
			if err != nil {
				return err
			}
			registry, err := recipes.Builtins()
			if err != nil {
				return err
			}
			implementation, ok := registry.Get(manifest.RecipeID)
			if !ok {
				return fmt.Errorf("backup recipe %q is not built into this Spare release", manifest.RecipeID)
			}
			values := manifest.Config
			if values == nil {
				values = map[string]any{}
			}
			if pathField := implementation.Manifest().Storage.PathField; pathField != "" {
				values[pathField] = absoluteDestination
			}
			port := manifest.Port
			portMode := manifest.PortMode
			if portValue != "" {
				port, portMode, err = parsePort(portValue)
				if err != nil {
					return err
				}
			}
			client, err := api.Discover(a.paths)
			if err != nil {
				return errors.New("backup data was restored, but Spare is not running. Run `spare init`, then install the restored folder")
			}
			current, err := client.Create(command.Context(), manifest.RecipeID, model.ModeInstalled, values, portMode, port)
			if err != nil {
				return fmt.Errorf("backup data was restored to %s, but the recipe was not installed: %w", absoluteDestination, err)
			}
			fmt.Fprintf(a.out, "Restored %s to %s.\n", sentenceCase(current.RecipeID), absoluteDestination)
			return nil
		},
	}
	command.Flags().StringVar(&destination, "path", "", "empty destination folder")
	command.Flags().StringVar(&portValue, "port", "", "override the saved port with auto or 1-65535")
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
				ok, err := confirm("Uninstall Spare? Selected folders will stay unchanged. [y/N] ")
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(a.out, "Spare was not uninstalled.")
					return nil
				}
			}
			if err := uninstall.Remove(command.Context(), a.paths); err != nil {
				return err
			}
			fmt.Fprintln(a.out, "Spare was removed. Selected folders were left unchanged.")
			return nil
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	return command
}

func (a *app) runnableManifest(reference string) (recipe.Manifest, error) {
	registry, err := recipes.Trusted()
	if err != nil {
		return recipe.Manifest{}, err
	}
	if implementation, ok := registry.Get(reference); ok {
		if reference != model.RecipeSite && reference != model.RecipeDrop && reference != model.RecipeHook {
			client, discoverErr := api.Discover(a.paths)
			if discoverErr != nil {
				return recipe.Manifest{}, errors.New("start Spare and install this optional job package first")
			}
			available, listErr := client.Recipes(context.Background())
			if listErr != nil {
				return recipe.Manifest{}, listErr
			}
			found := false
			for _, candidate := range available {
				if candidate.ID == reference {
					found = true
					break
				}
			}
			if !found {
				return recipe.Manifest{}, fmt.Errorf("%s is trusted but not installed; run `spare job add <package.sp>` first", implementation.Manifest().Name)
			}
		}
		return implementation.Manifest(), nil
	}
	manifest, err := recipe.Load(reference)
	if err != nil {
		return recipe.Manifest{}, fmt.Errorf("recipe %q is not built in and could not be loaded as a package: %w", reference, err)
	}
	implementation, ok := registry.Get(manifest.ID)
	if !ok {
		return recipe.Manifest{}, fmt.Errorf("%s is valid, but this Spare release does not include its trusted implementation", manifest.Name)
	}
	if !reflect.DeepEqual(manifest, implementation.Manifest()) {
		return recipe.Manifest{}, fmt.Errorf("%s is a valid package, but its manifest does not match the trusted built-in %s recipe", reference, manifest.Name)
	}
	compatibility := recipe.CurrentPlatformCompatible(manifest)
	if !compatibility.Supported {
		return recipe.Manifest{}, fmt.Errorf("%s is not compatible with this computer: %s", manifest.Name, strings.Join(compatibility.Reasons, " "))
	}
	return manifest, nil
}

func commandConfig(manifest recipe.Manifest, selectedPath, maximumFileSize string) (map[string]any, error) {
	values := map[string]any{}
	if manifest.Storage.PathField != "" {
		if selectedPath == "" {
			return nil, fmt.Errorf("choose %s with `--path <directory>`", strings.ToLower(manifest.Config[manifest.Storage.PathField].Label))
		}
		values[manifest.Storage.PathField] = selectedPath
	}
	if manifest.ID == model.RecipeDrop && maximumFileSize != "" {
		values["max-file-size"] = maximumFileSize
	}
	return values, nil
}

func printPermissions(output io.Writer, manifest recipe.Manifest) {
	statements := permissions.Describe(manifest.Permissions)
	fmt.Fprintf(output, "%s can:\n", manifest.Name)
	for _, statement := range statements {
		if statement.Granted {
			fmt.Fprintf(output, "  %s\n", statement.Description)
		}
	}
	fmt.Fprintf(output, "\n%s cannot:\n", manifest.Name)
	for _, statement := range statements {
		if !statement.Granted {
			fmt.Fprintf(output, "  %s\n", statement.Description)
		}
	}
	fmt.Fprintln(output)
}

func printDoctor(output io.Writer, report doctor.Report) {
	fmt.Fprintln(output, "Checking Spare...")
	fmt.Fprintln(output)
	for _, item := range report.Checks {
		fmt.Fprintf(output, "%-20s %-10s %s\n", item.Name, item.Status, item.Message)
		if item.Recovery != "" {
			fmt.Fprintf(output, "%-32s%s\n", "", item.Recovery)
		}
	}
}

func (a *app) waitForDaemon(ctx context.Context, timeout time.Duration) (*api.Client, error) {
	return bootstrap.WaitForDaemon(ctx, a.paths, timeout)
}

func findDaemon() (string, error) {
	return bootstrap.FindDaemon()
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

func printInstance(output io.Writer, heading, title string, instance model.Instance) {
	fmt.Fprintln(output, heading)
	fmt.Fprintln(output, "\nAvailable at")
	for _, url := range instance.URLs {
		fmt.Fprintln(output, url)
	}
	fmt.Fprintf(output, "\nNearby devices can open a LAN address while connected to the same network.\n%s data stays in:\n%s\n", title, instance.DataPath)
}

func titleForRecipe(recipes []model.Recipe, id string) string {
	for _, available := range recipes {
		if available.ID == id {
			return available.Title
		}
	}
	return sentenceCase(id)
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

func sentenceCase(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func formatError(err error) string {
	var clientError *api.ClientError
	if errors.As(err, &clientError) {
		if clientError.API.Hint != "" {
			return fmt.Sprintf("Error [%s]: %s\nRecovery: %s", clientError.API.Code, clientError.API.Message, clientError.API.Hint)
		}
		return fmt.Sprintf("Error [%s]: %s", clientError.API.Code, clientError.API.Message)
	}
	return err.Error()
}
