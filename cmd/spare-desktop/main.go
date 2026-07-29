//go:build desktop

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spare-run/spare/internal/bootstrap"
	"github.com/spare-run/spare/internal/dashboard"
	"github.com/spare-run/spare/internal/desktop"
	"github.com/spare-run/spare/internal/paths"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

var version = "dev"

func main() {
	statePaths, err := paths.Resolve()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	daemonPath, _ := bootstrap.FindDaemon()
	startHidden := false
	var launchPaths []string
	workingDirectory, _ := os.Getwd()
	for _, argument := range os.Args[1:] {
		if argument == "--hidden" {
			startHidden = true
		} else if argument != "" && argument[0] != '-' {
			launchPaths = append(launchPaths, resolveLaunchPath(argument, workingDirectory))
		}
	}
	application := desktop.New(statePaths, daemonPath, startHidden, launchPaths)

	err = wails.Run(&options.App{
		Title:             "Spare",
		Width:             930,
		Height:            509,
		MinWidth:          320,
		MinHeight:         480,
		StartHidden:       startHidden,
		HideWindowOnClose: true,
		BackgroundColour:  options.NewRGB(28, 28, 28),
		AssetServer: &assetserver.Options{
			Assets: dashboard.Files(),
		},
		OnStartup:     application.Startup,
		OnDomReady:    application.DomReady,
		OnShutdown:    application.Shutdown,
		OnBeforeClose: application.BeforeClose,
		Bind:          []any{application},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "run.spare.desktop",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				var paths []string
				for _, argument := range data.Args {
					if argument != "" && argument[0] != '-' {
						paths = append(paths, resolveLaunchPath(argument, data.WorkingDirectory))
					}
				}
				application.QueueLaunchPaths(paths)
			},
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		Mac: &mac.Options{
			TitleBar:   mac.TitleBarHidden(),
			Appearance: mac.NSAppearanceNameDarkAqua,
			About: &mac.AboutInfo{
				Title:   "Spare " + version,
				Message: "Give this computer a job.",
			},
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func resolveLaunchPath(path, workingDirectory string) string {
	if filepath.IsAbs(path) || workingDirectory == "" {
		return path
	}
	return filepath.Join(workingDirectory, path)
}
