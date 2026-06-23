// secscan — sec-scanner's custom image vulnerability scanner.
//
// secscan is a self-contained Go binary:
//   - pulls image layers from the registry via go-containerregistry (crane) and
//     flattens them in-process — no Docker daemon
//   - reads/writes the cvedb via a pure-Go SQLite driver (modernc.org/sqlite —
//     no cgo, no sqlite3 binary); the `import` subcommand replaced the Python importer
//   - talks to the ConfigHub REST API directly (CONFIGHUB_URL + CONFIGHUB_TOKEN),
//     the same surface the web app uses, to read fleet image refs and write
//     findings back — no `cub` shell-out
module secscan

go 1.25.0

require (
	github.com/google/go-containerregistry v0.21.6
	modernc.org/sqlite v1.52.0
)

require (
	github.com/docker/cli v29.4.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.3 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.6 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gotest.tools/v3 v3.5.2 // indirect
	modernc.org/libc v1.72.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)
