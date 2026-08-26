package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `epht - tools for impermanence ephemeral roots

usage:
  epht volatile [paths...]   live data that won't survive a reboot
  epht stale    [paths...]   persisted data no longer referenced by config
  epht status                summary of both, counts and sizes

flags (volatile/stale):
  -c, --collapse       collapse fully-unpersisted dirs to one line
      --size           on-disk size per entry plus a total
      --bytes          raw byte counts instead of human-readable
      --json           json array of {path, kind, size}, sizes in bytes
  -0, --null           nul-separated output, for xargs -0
  -m, --manifest PATH  use this manifest (repeatable), else $EPHT_MANIFEST

exit codes: 0 nothing found, 1 found something, 2 error
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "volatile", "stale":
		return cmdScan(args[0], args[1:], stdout, stderr)
	case "status":
		return cmdStatus(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "epht: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
