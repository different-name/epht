package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/different-name/keep-nix/internal/mounts"
	"github.com/different-name/keep-nix/internal/transfer"
)

// set at build time to coreutils' cp
var cpPath = "cp"

func cmdTransfer(name string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("keep "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var dryRun bool
	var manifests stringList
	fs.BoolVar(&dryRun, "dry-run", false, "print what would be copied")
	fs.BoolVar(&dryRun, "n", false, "shorthand for --dry-run")
	bindManifest(fs, &manifests)

	var positional []string
	for rest := args; ; {
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		positional = append(positional, rest[0])
		rest = rest[1:]
	}
	if len(positional) != 2 {
		fmt.Fprintf(stderr, "keep: usage: keep %s <path> <volume>\n", name)
		return 2
	}
	live, err := filepath.Abs(positional[0])
	if err != nil {
		return fail(stderr, err)
	}
	volume := positional[1]

	m, err := loadManifests(manifests)
	if err != nil {
		return fail(stderr, err)
	}
	table, err := mounts.Read()
	if err != nil {
		return fail(stderr, err)
	}
	passwd, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return fail(stderr, err)
	}
	env := transfer.Env{Manifest: m, Mounts: table, Homes: transfer.Homes(m, string(passwd))}

	plan, err := planFor(name, env, live, volume)
	if err != nil {
		return fail(stderr, err)
	}

	verb := "copied"
	if dryRun {
		verb = "would copy"
	}
	if !dryRun {
		if err := transfer.Execute(plan, cpPath); err != nil {
			return fail(stderr, err)
		}
	}
	fmt.Fprintf(stdout, "%s %s to %s\n", verb, plan.Source, plan.Target)
	if plan.Clear {
		if dryRun {
			fmt.Fprintf(stdout, "would empty %s, it becomes a mount point\n", plan.Source)
		} else {
			fmt.Fprintf(stdout, "emptied %s, it becomes a mount point\n", plan.Source)
		}
	}
	fmt.Fprintln(stdout, nextStep(name, plan, volume))
	return 0
}

func planFor(name string, env transfer.Env, live, volume string) (transfer.Plan, error) {
	if name == "adopt" {
		return transfer.Adopt(env, live, volume)
	}
	return transfer.Move(env, live, volume)
}

func nextStep(name string, p transfer.Plan, volume string) string {
	if name == "move" && !p.Clear {
		return fmt.Sprintf("next: declare %s in the %s volume and switch, then remove %s (keep list orphaned shows it)", p.Live, volume, p.Source)
	}
	return fmt.Sprintf("next: declare %s in the %s volume and switch", p.Live, volume)
}
