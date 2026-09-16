package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/thomasweidner/flashgate-mcp/internal/releaseevidence"
)

func main() {
	var o releaseevidence.Options
	flag.StringVar(&o.Artifact, "artifact", "", "validated release archive")
	flag.StringVar(&o.Checksum, "checksum", "", "archive SHA-256 file")
	flag.StringVar(&o.GoMod, "go-mod", "go.mod", "module manifest")
	flag.StringVar(&o.GoSum, "go-sum", "go.sum", "module checksums")
	flag.StringVar(&o.OutputDirectory, "output-directory", "", "evidence output directory")
	flag.StringVar(&o.Version, "version", "", "release version")
	flag.StringVar(&o.Commit, "commit", "", "full source commit")
	flag.StringVar(&o.SourceTime, "source-time", "", "canonical RFC3339 source time")
	flag.StringVar(&o.Platform, "platform", "", "release platform")
	flag.StringVar(&o.Architecture, "architecture", "", "public architecture")
	flag.Parse()
	paths, err := releaseevidence.Generate(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release evidence:", err)
		os.Exit(1)
	}
	for _, p := range paths {
		fmt.Println(p)
	}
}
