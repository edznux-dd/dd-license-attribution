// SPDX-License-Identifier: Apache-2.0
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/DataDog/dd-license-attribution/internal/adaptor"
	"github.com/DataDog/dd-license-attribution/internal/cleaner"
	"github.com/DataDog/dd-license-attribution/internal/port"
	"github.com/DataDog/dd-license-attribution/internal/report"
	"github.com/DataDog/dd-license-attribution/internal/strategy"
)

var cleanSPDXIDCmd = &cobra.Command{
	Use:   "clean-spdx-id <input-csv> <output-csv>",
	Short: "Convert long license texts to SPDX identifiers using an LLM",
	Long: `Reads a LICENSE-3rdparty.csv, identifies entries with long license texts,
and uses an LLM to convert them to standard SPDX identifiers.`,
	Args: cobra.ExactArgs(2),
	RunE: runCleanSPDXID,
}

var (
	llmProvider    string
	apiKey         string
	llmModel       string
	autoConfirm    bool
	cleanLogLevel  string
)

func init() {
	rootCmd.AddCommand(cleanSPDXIDCmd)

	f := cleanSPDXIDCmd.Flags()
	f.StringVar(&llmProvider, "llm-provider", "openai", "LLM provider (openai or anthropic)")
	f.StringVar(&apiKey, "api-key", "", "API key (falls back to OPENAI_API_KEY or ANTHROPIC_API_KEY)")
	f.StringVar(&llmModel, "model", "", "Specific model to use")
	f.BoolVarP(&autoConfirm, "yes", "y", false, "Auto-confirm all prompts")
	f.StringVar(&cleanLogLevel, "log-level", "INFO", "Logging level (DEBUG, INFO, WARNING, ERROR)")
}

func runCleanSPDXID(cmd *cobra.Command, args []string) error {
	inputCSV := args[0]
	outputCSV := args[1]

	configureLogging(cleanLogLevel)

	// Resolve API key
	key := apiKey
	if key == "" {
		switch llmProvider {
		case "openai":
			key = os.Getenv("OPENAI_API_KEY")
		case "anthropic":
			key = os.Getenv("ANTHROPIC_API_KEY")
		}
	}
	if key == "" {
		return fmt.Errorf("API key required. Provide via --api-key or set %s_API_KEY env var",
			fmt.Sprintf("%s", llmProvider))
	}

	// Create LLM client
	var llm port.LLMClient
	switch llmProvider {
	case "openai":
		model := llmModel
		if model == "" {
			model = "gpt-4"
		}
		llm = adaptor.NewOpenAIClient(key, model)
	case "anthropic":
		model := llmModel
		if model == "" {
			model = "claude-sonnet-4-5-20250929"
		}
		llm = adaptor.NewAnthropicClient(key, model)
	default:
		return fmt.Errorf("unsupported LLM provider: %s (use 'openai' or 'anthropic')", llmProvider)
	}

	fsys := &adaptor.RealFileSystem{}
	prompter := &adaptor.RealUserPrompter{}

	// Read input CSV
	csvStrategy := strategy.NewLicense3rdPartyStrategy(inputCSV, fsys)
	metas, err := csvStrategy.AugmentMetadata(nil)
	if err != nil {
		return fmt.Errorf("failed to read input CSV: %w", err)
	}

	// Set up change callback
	var callback cleaner.ChangeCallback
	if !autoConfirm {
		callback = func(info cleaner.ChangeInfo) bool {
			fmt.Fprintf(cmd.ErrOrStderr(), "\nComponent: %s\n", info.Component)
			fmt.Fprintf(cmd.ErrOrStderr(), "Original:  %s\n", info.Original)
			fmt.Fprintf(cmd.ErrOrStderr(), "Converted: %s\n", info.Converted)
			ok, err := prompter.Confirm("Accept this change?")
			if err != nil {
				return false
			}
			return ok
		}
	}

	// Run cleaner
	ctx := context.Background()
	spdxCleaner := cleaner.NewSPDXCleaner(llm)
	result, cleaned := spdxCleaner.CleanMetadata(ctx, metas, callback)

	// Write output CSV
	writer := &report.CSVWriter{}
	output := writer.Write(cleaned)
	if err := fsys.WriteFile(outputCSV, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write output CSV: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "\nProcessed %d rows, modified %d entries, %d changes\n",
		result.TotalRows, result.ModifiedCount, len(result.Changes))

	return nil
}
