package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/different-name/keep-nix/internal/manifest"
	"github.com/different-name/keep-nix/internal/scan"
)

type report struct {
	name    string
	results []scan.Result
}

type jsonItem struct {
	Report string `json:"report,omitempty"`
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Volume string `json:"volume,omitempty"`
	Size   int64  `json:"size"`
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

func emit(w io.Writer, reports []report, f listFlags, headed bool) error {
	switch {
	case f.summary:
		return emitSummary(w, reports, f)

	case f.json:
		items := []jsonItem{}
		for _, r := range reports {
			for _, res := range r.results {
				item := jsonItem{Path: res.Path, Kind: string(res.Kind), Volume: res.Volume, Size: scan.Size(res)}
				if headed {
					item.Report = r.name
				}
				items = append(items, item)
			}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(items)

	case f.null:
		for _, r := range reports {
			for _, res := range r.results {
				fmt.Fprintf(w, "%s\x00", res.Path)
			}
		}
		return nil
	}

	for i, r := range reports {
		if headed {
			if i > 0 {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "%s (%d)\n", r.name, len(r.results))
		}
		if f.size {
			emitSized(w, r.results, f.bytes)
		} else {
			for _, res := range r.results {
				fmt.Fprintln(w, res.Path)
			}
		}
	}
	return nil
}

func emitSized(w io.Writer, results []scan.Result, raw bool) {
	if len(results) == 0 {
		return
	}
	strs := make([]string, len(results))
	var total int64
	col := 0
	for i, r := range results {
		s := scan.Size(r)
		total += s
		strs[i] = sizeStr(s, raw)
		if len(strs[i]) > col {
			col = len(strs[i])
		}
	}
	if ts := sizeStr(total, raw); len(ts) > col {
		col = len(ts)
	}
	for i, r := range results {
		fmt.Fprintf(w, "%*s  %s\n", col, strs[i], r.Path)
	}
	fmt.Fprintf(w, "%*s  total\n", col, sizeStr(total, raw))
}

func emitDeclared(w io.Writer, m *manifest.Manifest, f listFlags) int {
	entries := append([]manifest.Entry(nil), m.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Volume != entries[j].Volume {
			return entries[i].Volume < entries[j].Volume
		}
		return entries[i].Live < entries[j].Live
	})

	if f.json {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(entries)
		return 0
	}
	if f.null {
		for _, e := range entries {
			fmt.Fprintf(w, "%s\x00", e.Live)
		}
		return 0
	}

	volume := ""
	for _, e := range entries {
		if e.Volume != volume {
			if volume != "" {
				fmt.Fprintln(w)
			}
			volume = e.Volume
			fmt.Fprintf(w, "%s (%s)\n", volume, m.Volumes[volume].Path)
		}
		fmt.Fprintf(w, "  %-4s %s\n", e.Kind, e.Live)
	}
	return 0
}

type summaryRow struct {
	Report string `json:"report"`
	Volume string `json:"volume,omitempty"`
	Count  int    `json:"count"`
	Size   int64  `json:"size"`
}

// orphaned data is deleted per volume, so it is split by volume
func summarize(reports []report) []summaryRow {
	var rows []summaryRow
	for _, r := range reports {
		byVolume := map[string]*summaryRow{}
		var order []string
		for _, res := range r.results {
			volume := ""
			if r.name == "orphaned" {
				volume = res.Volume
			}
			row, ok := byVolume[volume]
			if !ok {
				row = &summaryRow{Report: r.name, Volume: volume}
				byVolume[volume] = row
				order = append(order, volume)
			}
			row.Count++
			row.Size += scan.Size(res)
		}
		if len(order) == 0 {
			rows = append(rows, summaryRow{Report: r.name})
			continue
		}
		sort.Strings(order)
		for _, v := range order {
			rows = append(rows, *byVolume[v])
		}
	}
	return rows
}

func emitSummary(w io.Writer, reports []report, f listFlags) error {
	rows := summarize(reports)
	if f.json {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	labels := make([]string, len(rows))
	counts := make([]string, len(rows))
	sizes := make([]string, len(rows))
	var lw, cw, sw int
	for i, r := range rows {
		labels[i] = r.Report
		if r.Volume != "" {
			labels[i] += " " + r.Volume
		}
		counts[i] = fmt.Sprintf("%d", r.Count)
		sizes[i] = sizeStr(r.Size, f.bytes)
		lw, cw, sw = max(lw, len(labels[i])), max(cw, len(counts[i])), max(sw, len(sizes[i]))
	}
	for i := range rows {
		fmt.Fprintf(w, "%-*s  %*s paths  %*s\n", lw, labels[i], cw, counts[i], sw, sizes[i])
	}
	return nil
}
