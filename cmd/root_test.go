package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/youyo/tfstore/internal/backend"
)

// fakeBackend is a stdlib-only fake implementing backendAPI so tests never
// touch AWS credentials or the network.
type fakeBackend struct {
	createErr  error
	waitErr    error
	outputsBkt string
	outputsErr error

	createCalled     bool
	waitCalled       bool
	outputsCalled    bool
	createStackName  string
	waitStackName    string
	outputsStackName string
}

func (f *fakeBackend) Create(_ context.Context, stackName string) error {
	f.createCalled = true
	f.createStackName = stackName
	return f.createErr
}

func (f *fakeBackend) WaitForCreation(_ context.Context, stackName string) error {
	f.waitCalled = true
	f.waitStackName = stackName
	return f.waitErr
}

func (f *fakeBackend) GetOutputs(_ context.Context, stackName string) (string, error) {
	f.outputsCalled = true
	f.outputsStackName = stackName
	if f.outputsErr != nil {
		return "", f.outputsErr
	}
	return f.outputsBkt, nil
}

// stubNewBackend replaces the package-level newBackend factory for the
// duration of the test and restores the original afterward.
func stubNewBackend(t *testing.T, b backendAPI, region string, err error) {
	t.Helper()
	original := newBackend
	newBackend = func(context.Context) (backendAPI, string, error) {
		return b, region, err
	}
	t.Cleanup(func() { newBackend = original })
}

func TestFlagDefaults(t *testing.T) {
	cmd := newRootCmd()

	tests := []struct {
		name string
		want string
	}{
		{"stack-name", "tfstore"},
		{"region", ""},
		{"key", "terraform.tfstate"},
	}

	for _, tt := range tests {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Fatalf("flag %q not registered", tt.name)
		}
		if f.DefValue != tt.want {
			t.Errorf("flag %q default = %q, want %q", tt.name, f.DefValue, tt.want)
		}
	}
}

func TestFlagShorthands(t *testing.T) {
	cmd := newRootCmd()

	shorthands := map[string]string{"n": "stack-name", "r": "region", "k": "key"}
	for short, long := range shorthands {
		f := cmd.Flags().ShorthandLookup(short)
		if f == nil {
			t.Fatalf("shorthand -%s not registered", short)
		}
		if f.Name != long {
			t.Errorf("-%s maps to %q, want %q", short, f.Name, long)
		}
	}
}

func TestUsage_NoDynamoDBMentionOrTypo(t *testing.T) {
	cmd := newRootCmd()

	text := strings.ToLower(cmd.Short + " " + cmd.UsageString())
	if strings.Contains(text, "dyanamodb") {
		t.Errorf("usage text contains the Dyanamodb typo: %s", text)
	}
	if strings.Contains(text, "dynamodb") {
		t.Errorf("usage text still mentions DynamoDB: %s", text)
	}
}

func TestExecute_Success(t *testing.T) {
	fb := &fakeBackend{outputsBkt: "my-bucket"}
	stubNewBackend(t, fb, "ap-northeast-1", nil)

	cmd := newRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"--stack-name", "my-stack", "--key", "state.tfstate"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if !fb.createCalled || fb.createStackName != "my-stack" {
		t.Errorf("Create called=%v stackName=%q, want called with %q", fb.createCalled, fb.createStackName, "my-stack")
	}
	if !fb.waitCalled || fb.waitStackName != "my-stack" {
		t.Errorf("WaitForCreation called=%v stackName=%q, want called with %q", fb.waitCalled, fb.waitStackName, "my-stack")
	}
	if !fb.outputsCalled {
		t.Error("GetOutputs was not called")
	}

	want := backend.BackendConfigExample("my-bucket", "ap-northeast-1", "state.tfstate")
	if got := out.String(); !strings.Contains(got, want) {
		t.Errorf("output = %q, want it to contain the backend-config example:\n%s", got, want)
	}
}

func TestExecute_RegionFlagOverridesResolvedRegion(t *testing.T) {
	fb := &fakeBackend{outputsBkt: "my-bucket"}
	stubNewBackend(t, fb, "us-east-1", nil)

	cmd := newRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"--region", "eu-west-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	want := backend.BackendConfigExample("my-bucket", "eu-west-1", "terraform.tfstate")
	if got := out.String(); !strings.Contains(got, want) {
		t.Errorf("output = %q, want region override %q in:\n%s", got, "eu-west-1", want)
	}
}

func TestExecute_UsesResolvedRegionWhenFlagEmpty(t *testing.T) {
	fb := &fakeBackend{outputsBkt: "my-bucket"}
	stubNewBackend(t, fb, "ap-northeast-1", nil)

	cmd := newRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	want := backend.BackendConfigExample("my-bucket", "ap-northeast-1", "terraform.tfstate")
	if got := out.String(); !strings.Contains(got, want) {
		t.Errorf("output = %q, want resolved region %q in:\n%s", got, "ap-northeast-1", want)
	}
}

func TestExecute_PropagatesNewBackendError(t *testing.T) {
	wantErr := errors.New("no AWS credentials")
	stubNewBackend(t, nil, "", wantErr)

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestExecute_PropagatesCreateError(t *testing.T) {
	wantErr := errors.New("stack already exists")
	fb := &fakeBackend{createErr: wantErr}
	stubNewBackend(t, fb, "ap-northeast-1", nil)

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want it to wrap %v", err, wantErr)
	}
	if fb.waitCalled {
		t.Error("WaitForCreation must not be called when Create fails")
	}
}

func TestExecute_PropagatesWaitForCreationError(t *testing.T) {
	wantErr := errors.New("timed out waiting for CREATE_COMPLETE")
	fb := &fakeBackend{waitErr: wantErr}
	stubNewBackend(t, fb, "ap-northeast-1", nil)

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want it to wrap %v", err, wantErr)
	}
	if fb.outputsCalled {
		t.Error("GetOutputs must not be called when WaitForCreation fails")
	}
}

func TestExecute_PropagatesGetOutputsError(t *testing.T) {
	wantErr := errors.New("stack has no BucketName output")
	fb := &fakeBackend{outputsErr: wantErr}
	stubNewBackend(t, fb, "ap-northeast-1", nil)

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want it to wrap %v", err, wantErr)
	}
}
