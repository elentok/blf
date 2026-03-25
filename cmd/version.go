package cmd

import (
	"fmt"
	"io"
	"runtime/debug"
)

// version is set at build time via -ldflags "-X github.com/elentok/blf/cmd.version=vX.Y.Z".
var version = ""

func getVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func runVersion(w io.Writer) error {
	_, err := fmt.Fprintf(w, "blf %s\n", getVersion())
	return err
}
