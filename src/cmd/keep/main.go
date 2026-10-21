package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `keep - persisted state on an ephemeral root

usage:
  keep list                     unkept and orphaned, with headings
  keep list unkept   [paths]    live data that is lost at reboot
  keep list orphaned [paths]    store data no entry claims
  keep list declared            every entry with its volume and store
  keep check                    verify mounts, links and ownership

flags (list unkept, list orphaned):
  -f, --files          every file instead of one line per fully unkept or orphaned dir
      --size           on-disk size per entry plus a total
      --bytes          raw byte counts instead of human-readable
      --json           json array of {report, path, kind, volume, size}, sizes in bytes
  -0, --null           nul-separated paths, for xargs -0

flags (list declared):
      --json           json array of manifest entries
  -0, --null           nul-separated live paths

flags (all):
  -m, --manifest PATH  use this manifest (repeatable), else $KEEP_MANIFEST

exit codes: 0 nothing to report, 1 found something, 2 error
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
	case "list":
		return cmdList(args[1:], stdout, stderr)
	case "check":
		return cmdCheck(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "keep: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
