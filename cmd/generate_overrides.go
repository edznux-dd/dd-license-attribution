// SPDX-License-Identifier: Apache-2.0
package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DataDog/dd-license-attribution/internal/adaptor"
	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/overrides"
	"github.com/DataDog/dd-license-attribution/internal/strategy"
)

var generateOverridesCmd = &cobra.Command{
	Use:   "generate-overrides <csv-file>",
	Short: "Generate override rules from a LICENSE-3rdparty.csv",
	Long: `Interactively generates a JSON override rules file from a CSV with
problematic entries (missing license or copyright information).`,
	Args: cobra.ExactArgs(1),
	RunE: runGenerateOverrides,
}

var (
	overridesOutput    string
	onlyLicense        bool
	onlyCopyright      bool
	overridesLogLevel  string
)

func init() {
	rootCmd.AddCommand(generateOverridesCmd)

	f := generateOverridesCmd.Flags()
	f.StringVarP(&overridesOutput, "output", "o", ".ddla-overrides", "Output JSON file path")
	f.BoolVar(&onlyLicense, "only-license", false, "Only check for missing license info")
	f.BoolVar(&onlyCopyright, "only-copyright", false, "Only check for missing copyright info")
	f.StringVar(&overridesLogLevel, "log-level", "INFO", "Logging level (DEBUG, INFO, WARNING, ERROR)")

	generateOverridesCmd.MarkFlagsMutuallyExclusive("only-license", "only-copyright")
}

func runGenerateOverrides(cmd *cobra.Command, args []string) error {
	csvFile := args[0]

	configureLogging(overridesLogLevel)

	fsys := &adaptor.RealFileSystem{}
	prompter := &adaptor.RealUserPrompter{}

	// Read CSV to get current metadata
	csvStrategy := strategy.NewLicense3rdPartyStrategy(csvFile, fsys)
	metas, err := csvStrategy.AugmentMetadata(nil)
	if err != nil {
		return fmt.Errorf("failed to read CSV file: %w", err)
	}

	// Filter problematic entries
	var problematic []metadata.Metadata
	for _, m := range metas {
		missingLicense := len(m.License) == 0
		missingCopyright := len(m.Copyright) == 0

		if onlyLicense && !missingLicense {
			continue
		}
		if onlyCopyright && !missingCopyright {
			continue
		}
		if !onlyLicense && !onlyCopyright && !missingLicense && !missingCopyright {
			continue
		}
		problematic = append(problematic, m)
	}

	if len(problematic) == 0 {
		slog.Info("No problematic entries found")
		return nil
	}

	slog.Info("Found problematic entries", "count", len(problematic))

	// Collect override rules interactively
	var rules []strategy.OverrideRule
	for _, m := range problematic {
		fmt.Fprintf(cmd.ErrOrStderr(), "\n--- %s ---\n", m.Name)
		fmt.Fprintf(cmd.ErrOrStderr(), "  Origin:    %s\n", m.Origin)
		fmt.Fprintf(cmd.ErrOrStderr(), "  License:   %v\n", m.License)
		fmt.Fprintf(cmd.ErrOrStderr(), "  Copyright: %v\n", m.Copyright)

		fix, err := prompter.Confirm("Fix this entry?")
		if err != nil {
			return err
		}
		if !fix {
			continue
		}

		origin := m.Origin
		newOrigin, err := prompter.Prompt("Origin", origin)
		if err != nil {
			return err
		}

		licenseStr := strings.Join(m.License, ", ")
		newLicenseStr, err := prompter.Prompt("License(s) (comma-separated)", licenseStr)
		if err != nil {
			return err
		}
		var newLicense []string
		for _, l := range strings.Split(newLicenseStr, ",") {
			l = strings.TrimSpace(l)
			if l != "" {
				newLicense = append(newLicense, l)
			}
		}

		copyrightStr := strings.Join(m.Copyright, ", ")
		newCopyrightStr, err := prompter.Prompt("Copyright(s) (comma-separated)", copyrightStr)
		if err != nil {
			return err
		}
		var newCopyright []string
		for _, c := range strings.Split(newCopyrightStr, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				newCopyright = append(newCopyright, c)
			}
		}

		replacement := metadata.NewMetadata(m.Name, m.Version, newOrigin, "", newLicense, newCopyright)
		target := map[strategy.OverrideTargetField]string{
			strategy.TargetComponent: m.Name,
		}
		if m.Origin != "" {
			target[strategy.TargetOrigin] = m.Origin
		}

		rules = append(rules, strategy.OverrideRule{
			Type:        strategy.OverrideReplace,
			Target:      target,
			Replacement: &replacement,
		})
	}

	if len(rules) == 0 {
		slog.Info("No overrides generated")
		return nil
	}

	// Write overrides
	gen := overrides.NewGenerator()
	output, err := gen.Generate(rules)
	if err != nil {
		return fmt.Errorf("failed to generate overrides: %w", err)
	}

	if err := fsys.WriteFile(overridesOutput, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write overrides file: %w", err)
	}

	slog.Info("Overrides written", "file", overridesOutput, "count", len(rules))
	return nil
}
