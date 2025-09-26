package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ambiware-labs/loqa-core/internal/skills/manifest"
)

var version = "0.1.0-dev"

func main() {
	var manifestPath string
	validateCmd := flag.NewFlagSet("validate", flag.ExitOnError)
	validateCmd.StringVar(&manifestPath, "file", "skill.yaml", "Path to skill manifest")

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "expected 'validate' or 'version'")
		os.Exit(2)
	}

	switch os.Args[1] {
	case "validate":
		validateCmd.Parse(os.Args[2:])
		if err := runValidate(manifestPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("manifest valid")
	case "version":
		fmt.Println(version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
}

func runValidate(path string) error {
	m, err := manifest.Load(path)
	if err != nil {
		return err
	}
	return manifest.Validate(m)
}
