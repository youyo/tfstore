// Command tfstore creates a Terraform remote-state backend (an S3 bucket
// with native locking) via CloudFormation. See LICENSE for licensing terms.
package main

import (
	"os"
	"runtime/debug"

	"github.com/youyo/tfstore/cmd"
)

// version is the single source of truth for the CLI's version string. It is
// managed entirely via git tags: GoReleaser injects the release tag at
// build time via -ldflags "-X main.version={{.Version}}" (see
// .goreleaser.yml). Do not hardcode a version elsewhere.
var version = "dev"

// resolveVersion determines the effective CLI version from, in order:
//  1. the ldflags-injected version, when it was actually set at build time;
//  2. the module version reported by runtime/debug.ReadBuildInfo (e.g. for
//     `go install github.com/youyo/tfstore@vX.Y.Z` builds without ldflags);
//  3. "dev" as the final fallback.
//
// readBuildInfo is injected so tests can exercise the fallback paths
// deterministically instead of depending on how `go test` itself was built.
func resolveVersion(ldflagsVersion string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	if ldflagsVersion != "" && ldflagsVersion != "dev" {
		return ldflagsVersion
	}
	if info, ok := readBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func main() {
	cmd.SetVersion(resolveVersion(version, debug.ReadBuildInfo))
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
