// Command aileron-codegen scaffolds an Aileron connector from an OpenAPI or
// GraphQL specification plus a small gen.yaml overlay describing
// per-operation governance metadata.
//
//	aileron-codegen --spec <spec.{yaml,graphql}> --overlay <gen.yaml> --out <dir>
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ALRubinger/aileron-codegen/pkg/codegen"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("aileron-codegen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	specPath := fs.String("spec", "", "path to OpenAPI or GraphQL spec (required)")
	overlayPath := fs.String("overlay", "", "path to gen.yaml overlay (required)")
	outDir := fs.String("out", "", "output directory for generated files (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *specPath == "" || *overlayPath == "" || *outDir == "" {
		fs.Usage()
		return fmt.Errorf("--spec, --overlay, and --out are required")
	}
	return codegen.Generate(codegen.Options{
		SpecPath:    *specPath,
		OverlayPath: *overlayPath,
		OutDir:      *outDir,
	})
}
