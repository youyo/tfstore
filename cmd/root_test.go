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
	createInput      backend.CreateInput
	waitStackName    string
	outputsStackName string
}

func (f *fakeBackend) Create(_ context.Context, in backend.CreateInput) error {
	f.createCalled = true
	f.createInput = in
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

// countingNewBackend wraps a stub factory and counts how many times it was
// invoked, so tests can assert validation happens before any AWS call.
func countingNewBackend(b backendAPI, region string, err error) (*int, func(context.Context) (backendAPI, string, error)) {
	calls := 0
	return &calls, func(context.Context) (backendAPI, string, error) {
		calls++
		return b, region, err
	}
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
		{"stack-name", ""},
		{"bucket-name", ""},
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

	shorthands := map[string]string{"r": "region", "k": "key"}
	for short, long := range shorthands {
		f := cmd.Flags().ShorthandLookup(short)
		if f == nil {
			t.Fatalf("shorthand -%s not registered", short)
		}
		if f.Name != long {
			t.Errorf("-%s maps to %q, want %q", short, f.Name, long)
		}
	}

	if f := cmd.Flags().ShorthandLookup("n"); f != nil {
		t.Errorf("shorthand -n must be removed, but maps to %q", f.Name)
	}

	if f := cmd.Flags().Lookup("stack-name"); f != nil && f.Shorthand != "" {
		t.Errorf("--stack-name must be long-only, got shorthand %q", f.Shorthand)
	}
	if f := cmd.Flags().Lookup("bucket-name"); f != nil && f.Shorthand != "" {
		t.Errorf("--bucket-name must be long-only, got shorthand %q", f.Shorthand)
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

func TestRequiresNameArg(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error when no positional name is given")
	}
}

func TestStackNameDefault(t *testing.T) {
	fb := &fakeBackend{outputsBkt: "my-bucket"}
	stubNewBackend(t, fb, "ap-northeast-1", nil)

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"foo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	want := backend.CreateInput{StackName: "tfstore-foo", Name: "foo", BucketName: ""}
	if fb.createInput != want {
		t.Errorf("Create called with %+v, want %+v", fb.createInput, want)
	}
	if fb.waitStackName != "tfstore-foo" {
		t.Errorf("WaitForCreation stackName = %q, want %q", fb.waitStackName, "tfstore-foo")
	}
}

func TestStackNameOverride(t *testing.T) {
	fb := &fakeBackend{outputsBkt: "my-bucket"}
	stubNewBackend(t, fb, "ap-northeast-1", nil)

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"foo", "--stack-name", "bar"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	want := backend.CreateInput{StackName: "bar", Name: "foo", BucketName: ""}
	if fb.createInput != want {
		t.Errorf("Create called with %+v, want %+v", fb.createInput, want)
	}
}

func TestBucketNameOverride(t *testing.T) {
	fb := &fakeBackend{outputsBkt: "my-bucket"}
	stubNewBackend(t, fb, "ap-northeast-1", nil)

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"foo", "--bucket-name", "my-bkt"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	want := backend.CreateInput{StackName: "tfstore-foo", Name: "foo", BucketName: "my-bkt"}
	if fb.createInput != want {
		t.Errorf("Create called with %+v, want %+v", fb.createInput, want)
	}
}

func TestBothOverridesKeepName(t *testing.T) {
	fb := &fakeBackend{outputsBkt: "my-bucket"}
	stubNewBackend(t, fb, "ap-northeast-1", nil)

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"foo", "--stack-name", "bar", "--bucket-name", "baz"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	want := backend.CreateInput{StackName: "bar", Name: "foo", BucketName: "baz"}
	if fb.createInput != want {
		t.Errorf("Create called with %+v, want %+v", fb.createInput, want)
	}
}

func TestNameValidationRejectsInvalid(t *testing.T) {
	invalidNames := []string{
		"Foo",                   // uppercase
		"foo_bar",               // underscore
		strings.Repeat("a", 27), // >26 chars
	}

	for _, name := range invalidNames {
		fb := &fakeBackend{outputsBkt: "my-bucket"}
		stubNewBackend(t, fb, "ap-northeast-1", nil)

		cmd := newRootCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{name})

		if err := cmd.Execute(); err == nil {
			t.Errorf("Execute() with name %q: error = nil, want an error", name)
		}
		if fb.createCalled {
			t.Errorf("Create must not be called for invalid name %q", name)
		}
	}
}

func TestBucketNameValidationRejectsInvalid(t *testing.T) {
	invalidBucketNames := []string{
		"My-Bucket", // uppercase
		"has_underscore",
		"ab",                    // too short
		strings.Repeat("a", 64), // too long
	}

	for _, bkt := range invalidBucketNames {
		fb := &fakeBackend{outputsBkt: "my-bucket"}
		stubNewBackend(t, fb, "ap-northeast-1", nil)

		cmd := newRootCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"foo", "--bucket-name", bkt})

		if err := cmd.Execute(); err == nil {
			t.Errorf("Execute() with bucket-name %q: error = nil, want an error", bkt)
		}
		if fb.createCalled {
			t.Errorf("Create must not be called for invalid bucket-name %q", bkt)
		}
	}
}

func TestNameBoundary(t *testing.T) {
	fb := &fakeBackend{outputsBkt: "my-bucket"}
	stubNewBackend(t, fb, "ap-northeast-1", nil)

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{strings.Repeat("a", 26)})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() with 26-char name: error = %v, want nil", err)
	}
	if !fb.createCalled {
		t.Error("Create must be called for a 26-char name")
	}
}

func TestNameBoundary_TooLong(t *testing.T) {
	fb := &fakeBackend{outputsBkt: "my-bucket"}
	stubNewBackend(t, fb, "ap-northeast-1", nil)

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{strings.Repeat("a", 27)})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() with 27-char name: error = nil, want an error")
	}
	if fb.createCalled {
		t.Error("Create must not be called for a 27-char name")
	}
}

func TestBucketNameBoundary(t *testing.T) {
	for _, n := range []int{3, 63} {
		fb := &fakeBackend{outputsBkt: "my-bucket"}
		stubNewBackend(t, fb, "ap-northeast-1", nil)

		cmd := newRootCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"foo", "--bucket-name", strings.Repeat("a", n)})

		if err := cmd.Execute(); err != nil {
			t.Errorf("Execute() with %d-char bucket-name: error = %v, want nil", n, err)
		}
		if !fb.createCalled {
			t.Errorf("Create must be called for a %d-char bucket-name", n)
		}
	}
}

func TestBucketNameBoundary_OutOfRange(t *testing.T) {
	for _, n := range []int{2, 64} {
		fb := &fakeBackend{outputsBkt: "my-bucket"}
		stubNewBackend(t, fb, "ap-northeast-1", nil)

		cmd := newRootCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"foo", "--bucket-name", strings.Repeat("a", n)})

		if err := cmd.Execute(); err == nil {
			t.Errorf("Execute() with %d-char bucket-name: error = nil, want an error", n)
		}
		if fb.createCalled {
			t.Errorf("Create must not be called for a %d-char bucket-name", n)
		}
	}
}

func TestValidationBeforeAWS(t *testing.T) {
	calls, factory := countingNewBackend(&fakeBackend{outputsBkt: "my-bucket"}, "ap-northeast-1", nil)
	original := newBackend
	newBackend = factory
	t.Cleanup(func() { newBackend = original })

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"Invalid_Name"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error for invalid name")
	}
	if *calls != 0 {
		t.Errorf("newBackend called %d times, want 0 (validation must run before any AWS call)", *calls)
	}
}

func TestExecute_Success(t *testing.T) {
	fb := &fakeBackend{outputsBkt: "my-bucket"}
	stubNewBackend(t, fb, "ap-northeast-1", nil)

	cmd := newRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"foo", "--stack-name", "my-stack", "--key", "state.tfstate"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if !fb.createCalled || fb.createInput.StackName != "my-stack" {
		t.Errorf("Create called=%v stackName=%q, want called with %q", fb.createCalled, fb.createInput.StackName, "my-stack")
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
	cmd.SetArgs([]string{"foo", "--region", "eu-west-1"})

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
	cmd.SetArgs([]string{"foo"})

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
	cmd.SetArgs([]string{"foo"})

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
	cmd.SetArgs([]string{"foo"})

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
	cmd.SetArgs([]string{"foo"})

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
	cmd.SetArgs([]string{"foo"})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want it to wrap %v", err, wantErr)
	}
}
