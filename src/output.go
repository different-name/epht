package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/different-name/epht/internal/scan"
)

type jsonItem struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Size int64  `json:"size"`
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func sizeStr(n int64, raw bool) string {
	if raw {
		return fmt.Sprintf("%d", n)
	}
	return humanSize(n)
}

func emit(w io.Writer, results []scan.Result, f scanFlags) error {
	switch {
	case f.json:
		items := make([]jsonItem, len(results))
		for i, r := range results {
			items[i] = jsonItem{Path: r.Path, Kind: string(r.Kind), Size: scan.Size(r)}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(items)

	case f.null:
		for _, r := range results {
			fmt.Fprintf(w, "%s\x00", r.Path)
		}
		return nil

	case f.size:
		strs := make([]string, len(results))
		var total int64
		col := 0
		for i, r := range results {
			s := scan.Size(r)
			total += s
			strs[i] = sizeStr(s, f.bytes)
			if len(strs[i]) > col {
				col = len(strs[i])
			}
		}
		if len(results) == 0 {
			return nil
		}
		if ts := sizeStr(total, f.bytes); len(ts) > col {
			col = len(ts)
		}
		for i, r := range results {
			fmt.Fprintf(w, "%*s  %s\n", col, strs[i], r.Path)
		}
		fmt.Fprintf(w, "%*s  total\n", col, sizeStr(total, f.bytes))
		return nil

	default:
		for _, r := range results {
			fmt.Fprintln(w, r.Path)
		}
		return nil
	}
}
