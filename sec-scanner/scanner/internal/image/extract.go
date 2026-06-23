// Package image pulls a container image's layers straight from the registry
// with go-containerregistry (crane), flattens them in-process, and digs the OS
// package databases out of the resulting filesystem. No Docker daemon is
// involved, so it works in plain containers and CI without privileges.
package image

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"secscan/internal/pkgdb"
)

// Image is the part of an image the scanner cares about.
type Image struct {
	Ref       string            // as requested, e.g. alpine:3.9
	Digest    string            // manifest digest (sha256:…) if resolvable
	OSID      string            // os-release ID: alpine | debian | ubuntu
	OSVersion string            // os-release VERSION_ID: 3.9 | 12 | 22.04
	Ecosystem string            // OSV ecosystem: Alpine:v3.9 | Debian:12 | Ubuntu:22.04
	Files     map[string][]byte // captured package-db files
}

// Extract pulls the image from its registry, flattens the layers, and captures
// just the files the scanner needs. A per-file cap keeps memory bounded.
func Extract(ref string) (*Image, error) {
	img := &Image{Ref: ref, Files: map[string][]byte{}}

	rImg, err := crane.Pull(ref,
		// Images are commonly multi-arch indexes; pick a concrete platform.
		crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: "amd64"}),
		// Pick up `docker login` / registry creds when present (anonymous otherwise).
		crane.WithAuthFromKeychain(authn.DefaultKeychain),
	)
	if err != nil {
		return nil, fmt.Errorf("pull %s: %w", ref, err)
	}
	if d, err := rImg.Digest(); err == nil {
		img.Digest = d.String()
	}

	want := map[string]bool{}
	for _, p := range pkgdb.WantPaths() {
		want[p] = true
	}

	// mutate.Extract returns the flattened (whiteout-resolved) root filesystem
	// as a tar stream — the registry equivalent of `docker export`.
	rc := mutate.Extract(rImg)
	defer rc.Close()
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read image filesystem: %w", err)
		}
		name := normalizePath(hdr.Name)
		if !want[name] {
			continue
		}
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, io.LimitReader(tr, 64<<20)); err != nil {
			return nil, err
		}
		img.Files[name] = buf.Bytes()
	}

	img.parseOSRelease()
	return img, nil
}

// normalizePath strips the leading "./" or "/" tar entries may carry, matching
// the unprefixed paths in pkgdb.WantPaths().
func normalizePath(name string) string {
	name = strings.TrimPrefix(name, "./")
	return strings.TrimPrefix(name, "/")
}

func (img *Image) parseOSRelease() {
	// /etc/os-release is frequently a symlink (no content in the flattened tar),
	// so fall back to the real file under /usr/lib.
	b, ok := img.Files[pkgdb.PathOSRelease]
	if !ok || len(b) == 0 {
		b, ok = img.Files[pkgdb.PathOSReleaseLib]
	}
	if !ok || len(b) == 0 {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"`)
		switch k {
		case "ID":
			img.OSID = v
		case "VERSION_ID":
			img.OSVersion = v
		}
	}
	img.Ecosystem = ecosystem(img.OSID, img.OSVersion)
}

// ecosystem maps os-release identity to the OSV ecosystem string the cvedb is
// keyed on.
func ecosystem(id, ver string) string {
	switch id {
	case "alpine":
		// Alpine OSV uses major.minor only, with a leading v: "Alpine:v3.9".
		parts := strings.Split(ver, ".")
		if len(parts) >= 2 {
			ver = parts[0] + "." + parts[1]
		}
		return "Alpine:v" + ver
	case "debian":
		// VERSION_ID is the major release number: "Debian:12".
		return "Debian:" + ver
	case "ubuntu":
		return "Ubuntu:" + ver + ":LTS"
	}
	return ""
}

// Packages returns the installed OS packages.
func (img *Image) Packages() []pkgdb.Package {
	return pkgdb.Parse(img.Files)
}
