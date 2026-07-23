package cmd

import (
	"context"
	"fmt"
	"regexp"

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
	Create(ctx context.Context, in backend.CreateInput) error
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

// nameMaxLength keeps the generated bucket name
// (tfstate-{name}-{account-id}-{region}) within the S3 63-character limit
// even for the longest current AWS region name (us-isof-south-1, 15 chars):
// "tfstate-" (8) + name (26) + "-" (1) + account-id (12) + "-" (1) + region
// (15) = 63.
const nameMaxLength = 26

// nameRegexp validates the required positional `name` argument: lowercase
// letters, digits, and hyphens, not starting or ending with a hyphen.
var nameRegexp = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// bucketNameRegexp validates an explicit --bucket-name override against a
// conservative subset of S3 bucket naming rules: lowercase letters, digits,
// and hyphens, not starting or ending with a hyphen. This intentionally does
// not implement the full S3 spec (no dot-separated labels, no check for
// reserved prefixes/suffixes like "xn--" or "-s3alias").
var bucketNameRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// validateName checks the required positional name against the charset and
// length rules that keep the CloudFormation-derived default bucket name
// (tfstate-{name}-{account-id}-{region}) within the S3 63-character limit.
func validateName(name string) error {
	if len(name) == 0 || len(name) > nameMaxLength {
		return fmt.Errorf("invalid name %q: must be 1-%d characters", name, nameMaxLength)
	}
	if !nameRegexp.MatchString(name) {
		return fmt.Errorf("invalid name %q: must contain only lowercase letters, digits, and hyphens, and must not start or end with a hyphen", name)
	}
	return nil
}

// validateBucketName checks an explicit --bucket-name override. An empty
// bucketName (i.e. the flag was not set) is always valid: it lets the
// CloudFormation template apply its own default.
func validateBucketName(bucketName string) error {
	if bucketName == "" {
		return nil
	}
	if len(bucketName) < 3 || len(bucketName) > 63 {
		return fmt.Errorf("invalid --bucket-name %q: must be 3-63 characters", bucketName)
	}
	if !bucketNameRegexp.MatchString(bucketName) {
		return fmt.Errorf("invalid --bucket-name %q: must contain only lowercase letters, digits, and hyphens, and must not start or end with a hyphen", bucketName)
	}
	return nil
}

// newRootCmd builds a fresh rootCmd. A constructor (rather than a
// package-level singleton) keeps flag state isolated between invocations,
// which matters for tests that exercise the command multiple times with
// different arguments.
func newRootCmd() *cobra.Command {
	var stackName, bucketName, region, key string

	cmd := &cobra.Command{
		Use:   "tfstore <name>",
		Short: "Create a Terraform remote-state backend (S3, native locking).",
		Long: `Create a Terraform remote-state backend (S3, native locking) via a single
CloudFormation stack.

<name> identifies the backend and is used to derive defaults for both the
CloudFormation stack name (tfstore-<name>) and the S3 bucket name
(tfstate-<name>-<account-id>-<region>, resolved inside CloudFormation). Use
--stack-name / --bucket-name to override either default explicitly.`,
		Args:    cobra.ExactArgs(1),
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(cmd, args[0], stackName, bucketName, region, key)
		},
	}

	cmd.Flags().StringVar(&stackName, "stack-name", "", "CloudFormation stack name (default: tfstore-<name>)")
	cmd.Flags().StringVar(&bucketName, "bucket-name", "", "S3 bucket name (default: tfstate-<name>-<account-id>-<region>, resolved in CloudFormation)")
	cmd.Flags().StringVarP(&region, "region", "r", "", "AWS region (default: resolved from AWS configuration)")
	cmd.Flags().StringVarP(&key, "key", "k", "terraform.tfstate", "Terraform state object key")

	return cmd
}

// runRoot validates name/bucketName, resolves the effective stack name,
// then drives New -> Create -> WaitForCreation -> GetOutputs ->
// BackendConfigExample and prints the result to cmd's configured stdout.
// All input validation happens before newBackend is called so an invalid
// name or --bucket-name never reaches AWS.
func runRoot(cmd *cobra.Command, name, stackName, bucketName, region, key string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if err := validateBucketName(bucketName); err != nil {
		return err
	}

	effectiveStackName := stackName
	if effectiveStackName == "" {
		effectiveStackName = "tfstore-" + name
	}

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

	in := backend.CreateInput{
		StackName:  effectiveStackName,
		Name:       name,
		BucketName: bucketName,
	}
	if err := b.Create(ctx, in); err != nil {
		return err
	}
	if err := b.WaitForCreation(ctx, effectiveStackName); err != nil {
		return err
	}
	bucket, err := b.GetOutputs(ctx, effectiveStackName)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\nbucket: %s\nregion: %s\nkey:    %s\n\n", bucket, effectiveRegion, key)
	fmt.Fprintln(out, "Terraform initialize command is")
	fmt.Fprintln(out)
	fmt.Fprintln(out, backend.BackendConfigExample(bucket, effectiveRegion, key))

	return nil
}

// Execute runs the root command and returns any error encountered.
func Execute() error {
	return newRootCmd().Execute()
}
