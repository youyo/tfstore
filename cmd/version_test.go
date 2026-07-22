package cmd

import "testing"

func TestSetVersion_UpdatesRootCommandVersion(t *testing.T) {
	original := version
	t.Cleanup(func() { SetVersion(original) })

	SetVersion("v1.2.3")
	got := newRootCmd().Version
	if got != "v1.2.3" {
		t.Errorf("newRootCmd().Version = %q, want %q", got, "v1.2.3")
	}
}

func TestVersion_DefaultsToDevWhenUnset(t *testing.T) {
	original := version
	t.Cleanup(func() { SetVersion(original) })

	SetVersion("dev")
	got := newRootCmd().Version
	if got != "dev" {
		t.Errorf("newRootCmd().Version = %q, want %q", got, "dev")
	}
}
