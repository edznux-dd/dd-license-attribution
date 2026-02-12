// SPDX-License-Identifier: Apache-2.0
package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/DataDog/dd-license-attribution/internal/adaptor"
	"github.com/DataDog/dd-license-attribution/internal/artifact"
	"github.com/DataDog/dd-license-attribution/internal/collector"
	"github.com/DataDog/dd-license-attribution/internal/config"
	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/report"
	"github.com/DataDog/dd-license-attribution/internal/strategy"
)

var generateSBOMCSVCmd = &cobra.Command{
	Use:   "generate-sbom-csv <package>",
	Short: "Generate a LICENSE-3rdparty.csv from a GitHub repository",
	Long: `Generates a CSV report with license and copyright information for all
third-party dependencies of the specified GitHub repository.`,
	Args: cobra.ExactArgs(1),
	RunE: runGenerateSBOMCSV,
}

// Flags
var (
	deepScanning     bool
	onlyTransitive   bool
	onlyRootProject  bool
	noPyPI           bool
	noGoPkg          bool
	noGitHubSBOM     bool
	noNpm            bool
	noScancode       bool
	yarnSubdirs      []string
	cacheDir         string
	forceCacheCreate bool
	cacheTTL         int
	githubToken      string
	noGHAuth         bool
	overrideSpec     string
	useMirrors       string
	logLevel         string
)

func init() {
	rootCmd.AddCommand(generateSBOMCSVCmd)

	f := generateSBOMCSVCmd.Flags()
	f.BoolVar(&deepScanning, "deep-scanning", false, "Enable deep scanning")
	f.BoolVar(&onlyTransitive, "only-transitive-dependencies", false, "Only report transitive dependencies")
	f.BoolVar(&onlyRootProject, "only-root-project", false, "Only report root project")
	f.BoolVar(&noPyPI, "no-pypi-strategy", false, "Skip PyPI collection strategy")
	f.BoolVar(&noGoPkg, "no-gopkg-strategy", false, "Skip GoPkg collection strategy")
	f.BoolVar(&noGitHubSBOM, "no-github-sbom-strategy", false, "Skip GitHub SBOM collection strategy")
	f.BoolVar(&noNpm, "no-npm-strategy", false, "Skip NPM collection strategy")
	f.BoolVar(&noScancode, "no-scancode-strategy", false, "Skip license scan strategy")
	f.StringSliceVar(&yarnSubdirs, "yarn-subdir", nil, "Subdirectory paths for additional yarn.lock files")
	f.StringVar(&cacheDir, "cache-dir", "", "Directory for cached artifacts (temp dir if empty)")
	f.BoolVar(&forceCacheCreate, "force-cache-creation", false, "Force creation of cache directory")
	f.IntVar(&cacheTTL, "cache-ttl", config.DefaultCacheTTL, "Cache TTL in seconds")
	f.StringVar(&githubToken, "github-token", "", "GitHub authentication token (or GITHUB_TOKEN env)")
	f.BoolVar(&noGHAuth, "no-gh-auth", false, "Disable GitHub authentication")
	f.StringVar(&overrideSpec, "override-spec", "", "JSON file with override rules")
	f.StringVar(&useMirrors, "use-mirrors", "", "JSON file with mirror specifications")
	f.StringVar(&logLevel, "log-level", "INFO", "Logging level (DEBUG, INFO, WARNING, ERROR)")

	generateSBOMCSVCmd.MarkFlagsMutuallyExclusive("only-transitive-dependencies", "only-root-project")
}

func runGenerateSBOMCSV(cmd *cobra.Command, args []string) error {
	pkg := args[0]
	ctx := context.Background()

	// Configure logging
	configureLogging(logLevel)

	// Validate mutually exclusive flags
	if onlyTransitive && onlyRootProject {
		return fmt.Errorf("--only-transitive-dependencies and --only-root-project are mutually exclusive")
	}

	// Resolve GitHub token
	token := githubToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" && !noGHAuth {
		slog.Warn("No GitHub token provided. Using unauthenticated access (lower rate limits)")
	}

	// Set up IO adaptors
	fsys := &adaptor.RealFileSystem{}
	proc := &adaptor.RealProcessRunner{}
	clock := &adaptor.RealClock{}
	httpFetcher := adaptor.NewRealHTTPFetcher()
	detector := &adaptor.RealLicenseDetector{}

	var gh *adaptor.RealGitHubClient
	if !noGHAuth && token != "" {
		gh = adaptor.NewRealGitHubClient(token)
	} else {
		gh = adaptor.NewRealGitHubClient("")
	}

	// Set up cache directory
	effectiveCacheDir := cacheDir
	if effectiveCacheDir == "" {
		tmpDir, err := os.MkdirTemp("", "ddla-cache-*")
		if err != nil {
			return fmt.Errorf("failed to create temp cache directory: %w", err)
		}
		defer os.RemoveAll(tmpDir)
		effectiveCacheDir = tmpDir
	} else {
		effectiveCacheDir, _ = filepath.Abs(effectiveCacheDir)
		if forceCacheCreate {
			if err := os.MkdirAll(effectiveCacheDir, 0755); err != nil {
				return fmt.Errorf("failed to create cache directory: %w", err)
			}
		}
	}

	// Load mirrors
	var mirrors []artifact.MirrorSpec
	if useMirrors != "" {
		var err error
		mirrors, err = config.LoadMirrorConfigs(fsys, useMirrors)
		if err != nil {
			return err
		}
	}

	// Create source code manager
	scm, err := artifact.NewSourceCodeManager(effectiveCacheDir, gh, fsys, proc, clock, cacheTTL, mirrors)
	if err != nil {
		return fmt.Errorf("failed to create source code manager: %w", err)
	}

	// Determine project scope
	scope := metadata.ScopeAll
	if onlyRootProject {
		scope = metadata.ScopeOnlyRootProject
	} else if onlyTransitive {
		scope = metadata.ScopeOnlyTransitiveDependencies
	}

	// Load override rules
	var overrideRules []strategy.OverrideRule
	if overrideSpec != "" {
		overrideRules, err = config.LoadOverrideConfigs(fsys, overrideSpec)
		if err != nil {
			return err
		}
	}

	// Determine license/copyright file locations based on deep scanning
	var licenseFiles, copyrightFiles []string
	if !deepScanning {
		licenseFiles = config.PresetLicenseFileLocations
		copyrightFiles = config.PresetCopyrightFileLocations
	}

	// Build strategy chain
	var strategies []strategy.Strategy

	if !noGitHubSBOM {
		strategies = append(strategies,
			strategy.NewGitHubSBOMStrategy(ctx, gh, scm, scope, config.PresetCompanySuffixes))
	}
	if !noGoPkg {
		strategies = append(strategies,
			strategy.NewGoPkgStrategy(ctx, pkg, scm, scope, fsys, proc))
	}
	if !noPyPI {
		pyEnv := artifact.NewPythonEnvManager(effectiveCacheDir, fsys, proc, clock, cacheTTL)
		strategies = append(strategies,
			strategy.NewPyPIStrategy(ctx, pkg, scm, pyEnv, scope, config.PresetCompanySuffixes, httpFetcher, proc))
	}
	if !noNpm {
		strategies = append(strategies,
			strategy.NewNpmStrategy(ctx, pkg, scm, scope, yarnSubdirs, fsys, proc, httpFetcher))
	}
	if !noScancode {
		strategies = append(strategies,
			strategy.NewLicenseScanStrategy(ctx, scm, detector, fsys, licenseFiles, copyrightFiles))
	}

	// Always add GitHub repo strategy (fills in license/copyright from repo API)
	strategies = append(strategies,
		strategy.NewGitHubRepoStrategy(ctx, gh, scm))

	// Interleave override strategy before each main strategy
	if len(overrideRules) > 0 {
		overrideStrat := strategy.NewOverrideStrategy(overrideRules)
		interleaved := make([]strategy.Strategy, 0, len(strategies)*2)
		for _, s := range strategies {
			interleaved = append(interleaved, overrideStrat, s)
		}
		strategies = interleaved
	}

	// Always add cleanup copyright strategy at the end
	strategies = append(strategies, &strategy.CleanupCopyrightStrategy{})

	// Run the collection pipeline
	mc := collector.New(strategies)
	metas, err := mc.CollectMetadata(pkg)
	if err != nil {
		return fmt.Errorf("metadata collection failed: %w", err)
	}

	// Write CSV report
	writer := &report.CSVWriter{}
	output := writer.Write(metas)
	fmt.Print(output)

	return nil
}

func configureLogging(level string) {
	var logLevel slog.Level
	switch level {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "WARNING":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))
}
