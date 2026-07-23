// Package backend provisions and inspects the Terraform remote-state
// backend (an S3 bucket via a single CloudFormation stack).
package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/youyo/tfstore/internal/cfn"
)

// waitForCreationTimeout bounds how long WaitForCreation waits for a stack
// to reach CREATE_COMPLETE before giving up.
const waitForCreationTimeout = 5 * time.Minute

// bucketNameOutputKey is the CloudFormation Output key that carries the
// created bucket's name (see internal/cfn.Template).
const bucketNameOutputKey = "BucketName"

// Backend wraps a CloudFormation client for creating and inspecting the
// Terraform remote-state stack.
type Backend struct {
	Client CFNAPI
	Region string
}

// CFNAPI is the narrow CloudFormation surface Backend depends on. It is
// intentionally small enough for stdlib-only fakes and satisfies
// cloudformation.DescribeStacksAPIClient so it can be passed directly to
// SDK waiters.
type CFNAPI interface {
	CreateStack(ctx context.Context, params *cloudformation.CreateStackInput, optFns ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error)
	DescribeStacks(ctx context.Context, params *cloudformation.DescribeStacksInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error)
}

// New constructs a Backend using the default AWS SDK configuration chain.
func New(ctx context.Context) (*Backend, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &Backend{
		Client: cloudformation.NewFromConfig(cfg),
		Region: cfg.Region,
	}, nil
}

// nameParameterKey and bucketNameParameterKey are the CloudFormation
// Parameter names declared by internal/cfn.Template (the S1 contract).
const (
	nameParameterKey       = "Name"
	bucketNameParameterKey = "BucketName"
)

// CreateInput carries the values Create passes through to CloudFormation.
//
// Input validation (charset/length rules for Name and BucketName) is not
// Backend's responsibility: cmd validates both before constructing a
// Backend at all, so Create only enforces the pre-existing empty-StackName
// guard.
type CreateInput struct {
	// StackName is the CloudFormation stack name (a CreateStack API
	// argument; it never reaches the template).
	StackName string
	// Name is passed as the template's "Name" Parameter. It has no default
	// in the template, so it must always be supplied.
	Name string
	// BucketName is passed as the template's "BucketName" Parameter. An
	// empty value lets the template fall back to its deterministic default
	// derived from Name/account-id/region.
	BucketName string
}

// Create starts creation of the Terraform remote-state stack from the
// embedded CloudFormation template (internal/cfn.Template, the S1-foundation
// contract). It does not wait for the stack to finish creating; call
// WaitForCreation for that. No Capabilities are requested because the
// template declares no IAM resources.
//
// If a stack named in.StackName already exists, Create returns an error
// wrapping the underlying AlreadyExistsException. tfstore is a create-only
// tool: it does not support updating or migrating an existing stack.
func (b *Backend) Create(ctx context.Context, in CreateInput) error {
	if in.StackName == "" {
		return errors.New("backend: stack name must not be empty")
	}

	_, err := b.Client.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    aws.String(in.StackName),
		TemplateBody: aws.String(cfn.Template),
		Parameters: []types.Parameter{
			{ParameterKey: aws.String(nameParameterKey), ParameterValue: aws.String(in.Name)},
			{ParameterKey: aws.String(bucketNameParameterKey), ParameterValue: aws.String(in.BucketName)},
		},
	})
	if err != nil {
		var alreadyExists *types.AlreadyExistsException
		if errors.As(err, &alreadyExists) {
			return fmt.Errorf("backend: stack %q already exists; tfstore does not update or migrate existing stacks: %w", in.StackName, err)
		}
		return fmt.Errorf("backend: failed to create stack %q: %w", in.StackName, err)
	}
	return nil
}

// WaitForCreation blocks until stackName reaches CREATE_COMPLETE. It gives
// up after waitForCreationTimeout (5 minutes); the returned error includes
// the stack's last known status so the caller can see why creation did not
// finish.
func (b *Backend) WaitForCreation(ctx context.Context, stackName string) error {
	waiter := cloudformation.NewStackCreateCompleteWaiter(b.Client)
	err := waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, waitForCreationTimeout)
	if err == nil {
		return nil
	}

	status := "unknown"
	if out, describeErr := b.Client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}); describeErr == nil && len(out.Stacks) > 0 {
		status = string(out.Stacks[0].StackStatus)
	}

	return fmt.Errorf("backend: stack %q did not reach CREATE_COMPLETE (last status: %s): %w", stackName, status, err)
}

// GetOutputs returns the BucketName output of stackName. It returns an
// error if the stack cannot be found or has no BucketName output.
func (b *Backend) GetOutputs(ctx context.Context, stackName string) (string, error) {
	out, err := b.Client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return "", fmt.Errorf("backend: failed to describe stack %q: %w", stackName, err)
	}
	if len(out.Stacks) == 0 {
		return "", fmt.Errorf("backend: stack %q not found", stackName)
	}

	for _, o := range out.Stacks[0].Outputs {
		if o.OutputKey != nil && *o.OutputKey == bucketNameOutputKey && o.OutputValue != nil {
			return *o.OutputValue, nil
		}
	}

	return "", fmt.Errorf("backend: stack %q has no %s output", stackName, bucketNameOutputKey)
}

// BackendConfigExample renders the `terraform init` command a user should
// run against the created backend. It uses S3 native locking
// (use_lockfile=true) and intentionally has no dynamodb_table entry.
func BackendConfigExample(bucketName, region, key string) string {
	return fmt.Sprintf(`terraform init \
  -backend-config 'bucket=%s' \
  -backend-config 'key=%s' \
  -backend-config 'region=%s' \
  -backend-config 'encrypt=true' \
  -backend-config 'use_lockfile=true'`, bucketName, key, region)
}
