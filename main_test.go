package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion_LdflagsWins(t *testing.T) {
	got := resolveVersion("v1.2.3", func() (*debug.BuildInfo, bool) {
		t.Fatal("readBuildInfo should not be called when the ldflags version is set")
		return nil, false
	})
	if got != "v1.2.3" {
		t.Errorf("resolveVersion() = %q, want %q", got, "v1.2.3")
	}
}

func TestResolveVersion_FallsBackToBuildInfoVersion(t *testing.T) {
	got := resolveVersion("dev", func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v0.9.0"}}, true
	})
	if got != "v0.9.0" {
		t.Errorf("resolveVersion() = %q, want %q", got, "v0.9.0")
	}
}

func TestResolveVersion_DevelBuildInfoFallsBackToDev(t *testing.T) {
	got := resolveVersion("dev", func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true
	})
	if got != "dev" {
		t.Errorf("resolveVersion() = %q, want %q", got, "dev")
	}
}

func TestResolveVersion_NoBuildInfoFallsBackToDev(t *testing.T) {
	got := resolveVersion("dev", func() (*debug.BuildInfo, bool) {
		return nil, false
	})
	if got != "dev" {
		t.Errorf("resolveVersion() = %q, want %q", got, "dev")
	}
}

func TestResolveVersion_EmptyLdflagsFallsBackToDev(t *testing.T) {
	got := resolveVersion("", func() (*debug.BuildInfo, bool) {
		return nil, false
	})
	if got != "dev" {
		t.Errorf("resolveVersion() = %q, want %q", got, "dev")
	}
}
