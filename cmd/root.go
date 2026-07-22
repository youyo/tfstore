package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/youyo/tfstore/internal/backend"
)

// version is reported by --version. It defaults to "dev" and is expected to
// be set once by main (via SetVersion) before Execute is called; main owns
// version resolution (ldflags injection with a runtime/debug fallback) as
// the single source of truth (see main.go). Do not hardcode a version here.
var version = "dev"

// SetVersion sets the version string reported by --version. Call this from
// main before Execute.
func SetVersion(v string) {
	version = v
}

// backendAPI is the narrow surface RunE depends on for driving stack
// creation. *backend.Backend satisfies it; tests inject a fake via
// newBackend below.
type backendAPI interface {
	Create(ctx context.Context, stackName string) error
	WaitForCreation(ctx context.Context, stackName string) error
	GetOutputs(ctx context.Context, stackName string) (string, error)
}

// newBackend constructs the backendAPI used by RunE, along with the AWS
// region resolved from the default credentials chain. It is a package
// variable (rather than a direct call to backend.New) so tests can inject a
// fake without touching AWS credentials or the network.
var newBackend = func(ctx context.Context) (backendAPI, string, error) {
	b, err := backend.New(ctx)
	if err != nil {
		return nil, "", err
	}
	return b, b.Region, nil
}

// newRootCmd builds a fresh rootCmd. A constructor (rather than a
// package-level singleton) keeps flag state isolated between invocations,
// which matters for tests that exercise the command multiple times with
// different arguments.
func newRootCmd() *cobra.Command {
	var stackName, region, key string

	cmd := &cobra.Command{
		Use:     "tfstore",
		Short:   "Create a Terraform remote-state backend (S3, native locking).",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(cmd, stackName, region, key)
		},
	}

	cmd.Flags().StringVarP(&stackName, "stack-name", "n", "tfstore", "CloudFormation stack name")
	cmd.Flags().StringVarP(&region, "region", "r", "", "AWS region (default: resolved from AWS configuration)")
	cmd.Flags().StringVarP(&key, "key", "k", "terraform.tfstate", "Terraform state object key")

	return cmd
}

// runRoot drives New -> Create -> WaitForCreation -> GetOutputs ->
// BackendConfigExample and prints the result to cmd's configured stdout.
func runRoot(cmd *cobra.Command, stackName, region, key string) error {
	ctx := cmd.Context()

	b, resolvedRegion, err := newBackend(ctx)
	if err != nil {
		return err
	}

	effectiveRegion := region
	if effectiveRegion == "" {
		effectiveRegion = resolvedRegion
	}

	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Creating stack...")

	if err := b.Create(ctx, stackName); err != nil {
		return err
	}
	if err := b.WaitForCreation(ctx, stackName); err != nil {
		return err
	}
	bucketName, err := b.GetOutputs(ctx, stackName)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\nbucket: %s\nregion: %s\nkey:    %s\n\n", bucketName, effectiveRegion, key)
	fmt.Fprintln(out, "Terraform initialize command is")
	fmt.Fprintln(out)
	fmt.Fprintln(out, backend.BackendConfigExample(bucketName, effectiveRegion, key))

	return nil
}

// Execute runs the root command and returns any error encountered.
func Execute() error {
	return newRootCmd().Execute()
}
