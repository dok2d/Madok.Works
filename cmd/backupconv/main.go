// Command backupconv converts a pre-projects kanban backup (the old ExportAll
// JSON, optionally gzip-compressed) into the new project-scoped format, so it can
// be restored into a build that has the multi-project schema.
//
// It works on the generic JSON shape (map[string]any), not versioned structs, so
// it tolerates schema drift between the backup's origin and this tool. The
// transform is deliberately minimal:
//
//   - adds a top-level projects list with the default project "Общее" (id 1, open);
//   - stamps project_id=1 on every columns/epics/sprints/tags/tasks row;
//   - assigns each task a per-project sequential number (for KEY-<number> refs);
//   - stamps project_id=1 on ecosystem calendars and wiki_spaces rows.
//
// Usage:
//
//	backupconv -in old-export.json.gz -out new-export.json.gz
//	backupconv -in old.json            # writes <in>.converted.json.gz
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

func main() {
	in := flag.String("in", "", "input backup file (.json or .json.gz)")
	out := flag.String("out", "", "output file (default: <in> with .converted.json.gz)")
	flag.Parse()
	if *in == "" {
		fmt.Fprintln(os.Stderr, "usage: backupconv -in <old-backup.json[.gz]> [-out <new-backup.json.gz>]")
		os.Exit(2)
	}
	outPath := *out
	if outPath == "" {
		base := strings.TrimSuffix(strings.TrimSuffix(*in, ".gz"), ".json")
		outPath = base + ".converted.json.gz"
	}

	raw, err := os.ReadFile(*in)
	if err != nil {
		fail("read input: %v", err)
	}
	data, err := convert(gunzipIfNeeded(raw))
	if err != nil {
		fail("convert: %v", err)
	}
	if strings.HasSuffix(outPath, ".gz") {
		data = gzipBytes(data)
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		fail("write output: %v", err)
	}
	fmt.Printf("converted %s -> %s\n", *in, outPath)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "backupconv: "+format+"\n", args...)
	os.Exit(1)
}

// convert parses a backup's JSON and applies the project migration in place,
// returning the re-encoded JSON. Already-converted backups (with a projects key)
// are passed through unchanged.
func convert(jsonBytes []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(jsonBytes, &m); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	if _, done := m["projects"]; !done {
		m["projects"] = []any{map[string]any{
			"id":          1,
			"name":        "Общее",
			"key":         "GEN",
			"description": "",
			"visibility":  "open",
		}}
	}

	// Stamp project_id on the kanban entities.
	for _, key := range []string{"columns", "epics", "sprints", "tags", "tasks"} {
		for _, row := range asRows(m[key]) {
			row["project_id"] = 1
		}
	}

	// Assign per-project sequential task numbers (ordered by id for determinism).
	tasks := asRows(m["tasks"])
	sort.SliceStable(tasks, func(a, b int) bool { return numID(tasks[a]) < numID(tasks[b]) })
	for i, row := range tasks {
		row["number"] = i + 1
	}

	// Stamp project_id on ecosystem calendars and wiki spaces.
	if eco, ok := m["ecosystem"].(map[string]any); ok {
		for _, key := range []string{"calendars", "wiki_spaces"} {
			for _, row := range asRows(eco[key]) {
				row["project_id"] = 1
			}
		}
	}

	return json.MarshalIndent(m, "", "  ")
}

// asRows coerces a decoded JSON array into a slice of object maps, skipping
// anything that is not an object. Returns nil for a missing/!array value.
func asRows(v any) []map[string]any {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// numID returns a row's numeric "id" (JSON numbers decode as float64).
func numID(row map[string]any) float64 {
	if v, ok := row["id"].(float64); ok {
		return v
	}
	return 0
}

// gunzipIfNeeded transparently decompresses gzip payloads (by magic bytes), so
// both .json.gz and plain .json backups work.
func gunzipIfNeeded(b []byte) []byte {
	if len(b) < 2 || b[0] != 0x1f || b[1] != 0x8b {
		return b
	}
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return b
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		return b
	}
	return out
}

// gzipBytes compresses a payload at best compression.
func gzipBytes(b []byte) []byte {
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	zw.Write(b)
	zw.Close()
	return buf.Bytes()
}
