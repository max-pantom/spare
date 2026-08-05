package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spare-run/spare/internal/artifacts"
	"github.com/spare-run/spare/internal/jobpackage"
	"github.com/spare-run/spare/internal/permissions"
	"github.com/spare-run/spare/internal/recipe"
)

type catalog struct {
	Schema      string       `json:"schema"`
	GeneratedAt string       `json:"generatedAt"`
	Jobs        []catalogJob `json:"jobs"`
}

type catalogJob struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Status       string   `json:"status"`
	Wave         int      `json:"wave"`
	Version      string   `json:"version,omitempty"`
	MinimumSpare string   `json:"minimumSpareVersion,omitempty"`
	Publisher    string   `json:"publisher,omitempty"`
	Icon         string   `json:"icon"`
	Download     string   `json:"download,omitempty"`
	SHA256       string   `json:"sha256,omitempty"`
	Signature    string   `json:"signature,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
	Features     []string `json:"features"`
}

func main() {
	var output string
	var keyPath string
	var minimumSpare string
	flag.StringVar(&output, "output", "website", "catalog website directory")
	flag.StringVar(&keyPath, "key", "", "Ed25519 private key path")
	flag.StringVar(&minimumSpare, "minimum-spare-version", "0.1.1-alpha.3", "oldest compatible Spare release")
	flag.Parse()
	if keyPath == "" {
		keyPath = os.Getenv("SPARE_CATALOG_SIGNING_KEY")
	}
	if keyPath == "" {
		exit(errors.New("set SPARE_CATALOG_SIGNING_KEY or use --key"))
	}
	key, err := jobpackage.LoadPrivateKey(keyPath)
	if err != nil {
		exit(err)
	}
	downloads := filepath.Join(output, "downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		exit(err)
	}

	jobs := plannedJobs()
	for index := range jobs {
		if jobs[index].Status != "available" {
			continue
		}
		source := filepath.Join("recipes", jobs[index].ID)
		manifest, err := recipe.Load(source)
		if err != nil {
			exit(err)
		}
		name := fmt.Sprintf("%s_%s.sp", manifest.ID, manifest.Version)
		destination := filepath.Join(downloads, name)
		if _, err := recipe.Pack(source, destination); err != nil {
			exit(err)
		}
		envelope, err := jobpackage.Sign(destination, key, minimumSpare)
		if err != nil {
			exit(err)
		}
		if err := os.Chmod(destination, 0o644); err != nil {
			exit(err)
		}
		checksum, err := artifacts.SHA256(destination)
		if err != nil {
			exit(err)
		}
		statements := permissions.Describe(manifest.Permissions)
		var granted []string
		for _, statement := range statements {
			if statement.Granted {
				granted = append(granted, statement.Description)
			}
		}
		jobs[index].Version = manifest.Version
		jobs[index].MinimumSpare = minimumSpare
		jobs[index].Publisher = envelope.Publisher
		jobs[index].Download = "downloads/" + name
		jobs[index].SHA256 = checksum
		jobs[index].Signature = envelope.Signature
		jobs[index].Permissions = granted
	}
	value := catalog{
		Schema:      "spare.catalog/v1",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Jobs:        jobs,
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		exit(err)
	}
	data = append(data, '\n')
	path := filepath.Join(output, "catalog.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		exit(err)
	}
	fmt.Printf("Created %s with %d jobs\n", path, len(jobs))
}

func plannedJobs() []catalogJob {
	return []catalogJob{
		{
			ID:          "clipboard",
			Name:        "Clipboard",
			Description: "Move text, links, and small files between trusted devices.",
			Status:      "available",
			Wave:        1,
			Icon:        "icons/clipboard.svg",
			Features: []string{
				"Expiring text and links",
				"Small file sharing",
				"Trusted-device pairing",
			},
		},
		{
			ID:          "downloads",
			Name:        "Downloads",
			Description: "Download large files in the background.",
			Status:      "available",
			Wave:        1,
			Icon:        "icons/downloads.svg",
			Features: []string{
				"HTTP and HTTPS queue",
				"Pause, resume, and retry",
				"One transfer at a time",
			},
		},
		{
			ID:          "monitor",
			Name:        "Monitor",
			Description: "Know when websites, devices, or local services go offline.",
			Status:      "available",
			Wave:        1,
			Icon:        "icons/monitor.svg",
			Features: []string{
				"HTTP, ping, and TCP checks",
				"Response-time history",
				"Failure and recovery activity",
			},
		},
		{
			ID:          "archive",
			Name:        "Archive",
			Description: "Search old files across this computer and connected drives.",
			Status:      "planned",
			Wave:        2,
			Icon:        "icons/archive.svg",
			Features: []string{
				"Filename and document indexing",
				"Search across several drives",
				"Disconnected-drive awareness",
			},
		},
		{
			ID:          "media",
			Name:        "Media",
			Description: "Browse and play videos or audio from a folder or drive.",
			Status:      "planned",
			Wave:        2,
			Icon:        "icons/media.svg",
			Features: []string{
				"Local video and audio shelf",
				"Thumbnails and search",
				"Resume playback",
			},
		},
		{
			ID:          "dns",
			Name:        "DNS",
			Description: "Give local devices and services simple names.",
			Status:      "planned",
			Wave:        3,
			Icon:        "icons/dns.svg",
			Features: []string{
				"Friendly local names",
				"Device aliases",
				"Query and health activity",
			},
		},
		{
			ID:          "ad-blocker",
			Name:        "Ad Blocker",
			Description: "Block ads and trackers for devices on the network.",
			Status:      "planned",
			Wave:        3,
			Icon:        "icons/ad-blocker.svg",
			Features: []string{
				"DNS filtering",
				"Protected-device visibility",
				"Guided network setup",
			},
		},
		{
			ID:          "cameras",
			Name:        "Cameras",
			Description: "View and record trusted local cameras.",
			Status:      "planned",
			Wave:        4,
			Icon:        "icons/cameras.svg",
			Features: []string{
				"Trusted local streams",
				"Recording and retention",
				"Timelapse",
			},
		},
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
