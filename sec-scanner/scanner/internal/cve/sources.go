package cve

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// splitJSONArray returns each element of a JSON array as raw bytes; if the input
// is a single JSON object, it is returned as the only element.
func splitJSONArray(b []byte) [][]byte {
	var arr []json.RawMessage
	if err := json.Unmarshal(b, &arr); err == nil {
		out := make([][]byte, len(arr))
		for i, m := range arr {
			out[i] = m
		}
		return out
	}
	return [][]byte{b}
}

const osvBase = "https://osv-vulnerabilities.storage.googleapis.com"

// ReadOSVZip reads an OSV ecosystem export. spec is an ecosystem name
// ("Alpine:v3.9"), a full URL, or a local .zip path. Returns the label used in
// the import log and the parsed records.
func ReadOSVZip(spec string, limit int) (string, []Record, error) {
	var data []byte
	if strings.HasSuffix(spec, ".zip") {
		if b, err := os.ReadFile(spec); err == nil {
			data = b
		}
	}
	if data == nil {
		url := spec
		if !strings.HasPrefix(spec, "http") {
			url = fmt.Sprintf("%s/%s/all.zip", osvBase, spec)
		}
		fmt.Fprintf(os.Stderr, "  fetching %s\n", url)
		b, err := httpGet(url)
		if err != nil {
			return spec, nil, err
		}
		data = b
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return spec, nil, fmt.Errorf("open zip: %w", err)
	}
	var out []Record
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".json") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		if rec, ok := osvToNorm(b, "osv"); ok {
			out = append(out, rec)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return spec, out, nil
}

// ReadGHSA walks a clone of github/advisory-database (OSV files under advisories/).
func ReadGHSA(path string, limit int) (string, []Record, error) {
	root := path
	if fi, err := os.Stat(filepath.Join(path, "advisories")); err == nil && fi.IsDir() {
		root = filepath.Join(path, "advisories")
	}
	out, err := walkJSON(root, "GHSA-", limit, func(b []byte) (Record, bool) { return osvToNorm(b, "ghsa") })
	return root, out, err
}

// ReadCVEList walks a clone of CVEProject/cvelistV5 (CVE JSON 5.0).
func ReadCVEList(path string, limit int) (string, []Record, error) {
	out, err := walkJSON(path, "CVE-", limit, cve5ToNorm)
	return path, out, err
}

// ReadFixtures reads cvedb/fixtures/*.json (each an array or object of OSV docs).
func ReadFixtures(dir string, limit int) (string, []Record, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return dir, nil, err
	}
	var out []Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		// A fixtures file is a JSON array of OSV docs; split it and parse each.
		for _, doc := range splitJSONArray(b) {
			if rec, ok := osvToNorm(doc, "osv"); ok {
				out = append(out, rec)
			}
			if limit > 0 && len(out) >= limit {
				return dir, out, nil
			}
		}
	}
	return dir, out, nil
}

func walkJSON(root, prefix string, limit int, parse func([]byte) (Record, bool)) ([]Record, error) {
	var out []Record
	stop := fmt.Errorf("limit")
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".json") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		if rec, ok := parse(b); ok {
			out = append(out, rec)
		}
		if limit > 0 && len(out) >= limit {
			return stop
		}
		return nil
	})
	if err == stop {
		err = nil
	}
	return out, err
}

func httpGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
