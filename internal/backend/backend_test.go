package backend

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/youyo/tfstore/internal/cfn"
)

// fakeCFNAPI is a stdlib-only, function-field fake implementing CFNAPI so
// tests never touch AWS credentials or the network.
type fakeCFNAPI struct {
	createStackFn    func(ctx context.Context, params *cloudformation.CreateStackInput, optFns ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error)
	describeStacksFn func(ctx context.Context, params *cloudformation.DescribeStacksInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error)
}

func (f *fakeCFNAPI) CreateStack(ctx context.Context, params *cloudformation.CreateStackInput, optFns ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error) {
	return f.createStackFn(ctx, params, optFns...)
}

func (f *fakeCFNAPI) DescribeStacks(ctx context.Context, params *cloudformation.DescribeStacksInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	return f.describeStacksFn(ctx, params, optFns...)
}

func TestCreate_PassesTemplateAndStackNameWithoutCapabilities(t *testing.T) {
	var gotInput *cloudformation.CreateStackInput
	b := &Backend{Client: &fakeCFNAPI{
		createStackFn: func(_ context.Context, params *cloudformation.CreateStackInput, _ ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error) {
			gotInput = params
			return &cloudformation.CreateStackOutput{StackId: aws.String("stack-id")}, nil
		},
	}}

	in := CreateInput{StackName: "my-stack", Name: "foo", BucketName: ""}
	if err := b.Create(context.Background(), in); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	if gotInput == nil {
		t.Fatal("CreateStack was not called")
	}
	if got := aws.ToString(gotInput.StackName); got != in.StackName {
		t.Errorf("StackName = %q, want %q", got, in.StackName)
	}
	if got := aws.ToString(gotInput.TemplateBody); got != cfn.Template {
		t.Errorf("TemplateBody = %q, want internal/cfn.Template", got)
	}
	if len(gotInput.Capabilities) != 0 {
		t.Errorf("Capabilities = %v, want none", gotInput.Capabilities)
	}

	wantParams := map[string]string{"Name": in.Name, "BucketName": in.BucketName}
	gotParams := map[string]string{}
	for _, p := range gotInput.Parameters {
		gotParams[aws.ToString(p.ParameterKey)] = aws.ToString(p.ParameterValue)
	}
	if gotParams["Name"] != wantParams["Name"] {
		t.Errorf("Parameters[Name] = %q, want %q", gotParams["Name"], wantParams["Name"])
	}
	if gotParams["BucketName"] != wantParams["BucketName"] {
		t.Errorf("Parameters[BucketName] = %q, want %q", gotParams["BucketName"], wantParams["BucketName"])
	}
}

func TestCreate_PassesNonEmptyBucketNameOverride(t *testing.T) {
	var gotInput *cloudformation.CreateStackInput
	b := &Backend{Client: &fakeCFNAPI{
		createStackFn: func(_ context.Context, params *cloudformation.CreateStackInput, _ ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error) {
			gotInput = params
			return &cloudformation.CreateStackOutput{StackId: aws.String("stack-id")}, nil
		},
	}}

	in := CreateInput{StackName: "my-stack", Name: "foo", BucketName: "my-explicit-bucket"}
	if err := b.Create(context.Background(), in); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	gotParams := map[string]string{}
	for _, p := range gotInput.Parameters {
		gotParams[aws.ToString(p.ParameterKey)] = aws.ToString(p.ParameterValue)
	}
	if gotParams["Name"] != in.Name {
		t.Errorf("Parameters[Name] = %q, want %q", gotParams["Name"], in.Name)
	}
	if gotParams["BucketName"] != in.BucketName {
		t.Errorf("Parameters[BucketName] = %q, want %q", gotParams["BucketName"], in.BucketName)
	}
}

func TestCreate_RejectsEmptyStackName(t *testing.T) {
	b := &Backend{Client: &fakeCFNAPI{
		createStackFn: func(context.Context, *cloudformation.CreateStackInput, ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error) {
			t.Fatal("CreateStack must not be called for an empty stack name")
			return nil, nil
		},
	}}

	if err := b.Create(context.Background(), CreateInput{StackName: "", Name: "foo"}); err == nil {
		t.Fatal("Create() error = nil, want error for empty stack name")
	}
}

func TestCreate_AlreadyExists(t *testing.T) {
	b := &Backend{Client: &fakeCFNAPI{
		createStackFn: func(context.Context, *cloudformation.CreateStackInput, ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error) {
			return nil, &types.AlreadyExistsException{Message: aws.String("stack already exists")}
		},
	}}

	err := b.Create(context.Background(), CreateInput{StackName: "my-stack", Name: "foo"})
	if err == nil {
		t.Fatal("Create() error = nil, want an AlreadyExists error")
	}

	var alreadyExists *types.AlreadyExistsException
	if !errors.As(err, &alreadyExists) {
		t.Fatalf("Create() error = %v, want it to wrap *types.AlreadyExistsException", err)
	}
}

func TestCreate_WrapsOtherErrors(t *testing.T) {
	wantErr := errors.New("network is down")
	b := &Backend{Client: &fakeCFNAPI{
		createStackFn: func(context.Context, *cloudformation.CreateStackInput, ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error) {
			return nil, wantErr
		},
	}}

	err := b.Create(context.Background(), CreateInput{StackName: "my-stack", Name: "foo"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Create() error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestWaitForCreation_ImmediateSuccess(t *testing.T) {
	calls := 0
	b := &Backend{Client: &fakeCFNAPI{
		describeStacksFn: func(_ context.Context, params *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			calls++
			return &cloudformation.DescribeStacksOutput{
				Stacks: []types.Stack{{
					StackName:    params.StackName,
					StackStatus:  types.StackStatusCreateComplete,
					CreationTime: aws.Time(time.Now()),
				}},
			}, nil
		},
	}}

	if err := b.WaitForCreation(context.Background(), "my-stack"); err != nil {
		t.Fatalf("WaitForCreation() error = %v, want nil", err)
	}
	if calls != 1 {
		t.Errorf("DescribeStacks called %d time(s), want exactly 1 (immediate CREATE_COMPLETE)", calls)
	}
}

func TestGetOutputs_ReturnsBucketName(t *testing.T) {
	b := &Backend{Client: &fakeCFNAPI{
		describeStacksFn: func(context.Context, *cloudformation.DescribeStacksInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{
				Stacks: []types.Stack{{
					Outputs: []types.Output{
						{OutputKey: aws.String("BucketName"), OutputValue: aws.String("my-bucket")},
					},
				}},
			}, nil
		},
	}}

	got, err := b.GetOutputs(context.Background(), "my-stack")
	if err != nil {
		t.Fatalf("GetOutputs() error = %v, want nil", err)
	}
	if got != "my-bucket" {
		t.Errorf("GetOutputs() = %q, want %q", got, "my-bucket")
	}
}

func TestGetOutputs_ErrorsWhenBucketNameMissing(t *testing.T) {
	b := &Backend{Client: &fakeCFNAPI{
		describeStacksFn: func(context.Context, *cloudformation.DescribeStacksInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{
				Stacks: []types.Stack{{Outputs: nil}},
			}, nil
		},
	}}

	if _, err := b.GetOutputs(context.Background(), "my-stack"); err == nil {
		t.Fatal("GetOutputs() error = nil, want error when BucketName output is missing")
	}
}

func TestGetOutputs_ErrorsWhenStackNotFound(t *testing.T) {
	b := &Backend{Client: &fakeCFNAPI{
		describeStacksFn: func(context.Context, *cloudformation.DescribeStacksInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{Stacks: nil}, nil
		},
	}}

	if _, err := b.GetOutputs(context.Background(), "missing-stack"); err == nil {
		t.Fatal("GetOutputs() error = nil, want error when stack is not found")
	}
}

func TestGetOutputs_WrapsDescribeError(t *testing.T) {
	wantErr := errors.New("describe failed")
	b := &Backend{Client: &fakeCFNAPI{
		describeStacksFn: func(context.Context, *cloudformation.DescribeStacksInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return nil, wantErr
		},
	}}

	_, err := b.GetOutputs(context.Background(), "my-stack")
	if !errors.Is(err, wantErr) {
		t.Fatalf("GetOutputs() error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestBackendConfigExample_Golden(t *testing.T) {
	got := BackendConfigExample("my-bucket", "ap-northeast-1", "terraform.tfstate")
	want := `terraform init \
  -backend-config 'bucket=my-bucket' \
  -backend-config 'key=terraform.tfstate' \
  -backend-config 'region=ap-northeast-1' \
  -backend-config 'encrypt=true' \
  -backend-config 'use_lockfile=true'`

	if got != want {
		t.Errorf("BackendConfigExample() =\n%s\nwant:\n%s", got, want)
	}
}
