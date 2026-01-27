# Etherpad-Go

**A fast, modern, real-time collaborative editor written in Go**

[![CI](https://github.com/ether/etherpad-go/actions/workflows/build.yml/badge.svg)](https://github.com/ether/etherpad-go/actions)
[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/github/license/ether/etherpad-go)](LICENSE)
[![Release](https://img.shields.io/github/v/release/ether/etherpad-go)](https://github.com/ether/etherpad-go/releases)
[![Docker Image](https://img.shields.io/badge/docker-ghcr.io%2Fether%2Fetherpad--go-blue?logo=docker)](https://ghcr.io/ether/etherpad-go)

Etherpad-Go is a performance-focused, 1:1 rewrite of Etherpad-Lite in Go.
The original implementation was written in Node.js (CommonJS).
Rewriting Etherpad in Go allows us to leverage Go’s concurrency model,
static typing, fast startup times, and lower memory usage.

<p align="center">
  <img src="doc/public/gopherEtherpad.png"
       alt="Etherpad-Go Logo"
       title="Etherpad-Go Logo">
</p>

<p align="center">
  <img src="doc/public/etherpad_demo.gif"
       alt="Etherpad in action"
       title="Etherpad-Go demo">
</p>

Etherpad is a real-time collaborative editor
[scalable to thousands of simultaneous users](http://scale.etherpad.org/)
and supports
[full data export](https://github.com/ether/etherpad-lite/wiki/Understanding-Etherpad's-Full-Data-Export-capabilities).

---

## Quick Start (Binary)

The easiest way to run Etherpad-Go:

1. Download the binary for your platform from the  
   [Releases page](https://github.com/ether/etherpad-go/releases)
2. Run it:

```bash
./etherpad-go
```

Etherpad-Go starts in **under a second** and uses an **in-memory database
by default**, which is sufficient for many use cases.

Open your browser at:  
http://localhost:9001

---

## Configuration

All configuration options are self-documented in the binary.

Show global options:

```bash
./etherpad-go --help
```

Show all configuration values:

```bash
./etherpad-go config show
```

Get a specific configuration value:

```bash
./etherpad-go config get <key>
```

For customization, copy:

```text
settings.json.template → settings.json
```

and adjust it to your needs.

---

## Docker

You can run Etherpad-Go using Docker.

### Docker Compose (recommended)

Start Etherpad-Go **with PostgreSQL**:

```bash
docker compose up -d
```

Etherpad will be available at:  
http://localhost:9001

The compose file can be found here:  
[docker-compose.yml](./docker-compose.yml)

### Build your own image

```bash
docker build -t etherpad-go .
```

### Prebuilt images

Prebuilt images are available via GitHub Container Registry:

```text
ghcr.io/ether/etherpad-go:<version>
```

---

## Building from Source

### Requirements

- [Go 1.25+](https://golang.org/dl/)
- [Node.js 22+](https://nodejs.org/en/download/)
- [pnpm](https://pnpm.io/installation)

### Installation

1. Clone the repository:

```bash
git clone https://github.com/ether/etherpad-go.git
cd etherpad-go
```

2. Build the UI:

```bash
cd ui
pnpm install
cd ../admin
pnpm install
node build.js
```

3. Build the Go server:

```bash
go install github.com/a-h/templ/cmd/templ@latest
templ generate
go build -o etherpad-go ./main.go
```

4. Run the server:

```bash
./etherpad-go
```

Etherpad should start in less than a second.

---

## Migration from Etherpad-Lite

You can migrate existing pads from Etherpad-Lite using the `migration` command:

```bash
migration 192.28.91.4:5432 \
  --type postgres \
  --username myOldEtherpadDBUser \
  --database myoldEtherpadDB
```

This connects to the old Etherpad-Lite database and migrates all pads to
the Etherpad-Go database configured in `settings.json` or via environment
variables.

---

## Plugins

Etherpad-Go supports plugins, but the plugin system is still evolving.

Supported plugins can be found in the
[plugins directory](./plugins).

Enable plugins via `settings.json`:

```json
{
  "plugins": {
    "ep_align": { "enabled": true }
  }
}
```

Or via environment variables:

```text
ETHERPAD_PLUGINS_EP_ALIGN_ENABLED=true
```

---

## Status

Etherpad-Go is under active development.
Feedback, issues, and contributions are welcome.