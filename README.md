# Etherpad-Go: A real-time collaborative editor for the web

Etherpad-Go is a performance focused 1 to 1 rewrite of Etherpad-Lite in Go. The old implementation was written in NodeJS and was still in CommonJS. 
A new implementation in Go allows us to take advantage of Go's concurrency model, static typing, and performance optimizations.


![Demo Etherpad Animated Jif](doc/public/etherpad_demo.gif "Etherpad in action")


Etherpad is a real-time collaborative editor [scalable to thousands of
simultaneous real time users](http://scale.etherpad.org/). It provides [full
data export](https://github.com/ether/etherpad-lite/wiki/Understanding-Etherpad's-Full-Data-Export-capabilities)

## Config

All the configuration already documents in Etherpad-Go itself. You can just run

```bash
./etherpad-go --help
```

to find root level options. The sublevel commands e.g. like `./etherpad-go config` also have their own help pages.

## Requirements to run Etherpad-Go


* For a blank app. Just download the binary for your platform from the [releases page](https://github.com/ether/etherpad-go/releases) and run it. It uses an in memory database which should be more than enough. The base path is `var/etherpad.db` but again all the defaults are also shown with `./etherpad-go config show` or for a particular config `./etherpad-go config get <key>`.


## Requirements to get started (only for building from source)

* [Go 1.25+](https://golang.org/dl/)
* [NodeJS 22+](https://nodejs.org/en/download/)
* [Pnpm](https://pnpm.io/installation)


### Installation

1. Clone the repository 
   ```bash
   git clone https://github.com/ether/etherpad-go.git
    cd etherpad-go
    ```
   
2. Create the ui: 

    ```bash
    cd ui
    pnpm install
    cd ../admin
    pnpm install
    node build.js
    ```

3. Build the Go server:
    ```bash
    go build -o etherpad-go ./main.go
    ```
4. Copy the binary where you want to run it
5. For customization and configuration, copy the `settings.json.template` to `settings.json` and edit it to your needs.
6. Run the server:
    ```bash
    ./etherpad-go
    ```
7. Etherpad should start within less than a second. Open your browser and navigate to `http://localhost:9001` to access the Etherpad interface.

## Docker

You can also run Etherpad-Go using Docker. Here's how to do it:

### Build it your own

1. Build the Docker image:
   ```bash
   docker build -t etherpad-go .
   ```

2. Use a prebuilt one which is available at [GitHub Container Registry](ghcr.io/ether/etherpad-go:<your-desired-version>)

## Starting the Docker Container

You can start the Docker container with a postgres database with docker compose:

```bash
docker compose up -d
```
This will start both the Etherpad-Go server and a Postgres database. The server will be accessible at `http://localhost:9001`.
You can find the docker compose file in [docker-compose.yml](./docker-compose.yml)


# Importing things to note

You can import existing pads from Etherpad-Lite into Etherpad-Go. You can do this with the migrate command e.g.

```bash
migration 192.28.91.4:5432 --type postgres --username myOldEtherpadDBUser --database myoldEtherpadDB
```

This will connect to the old Etherpad-Lite database and migrate all pads to the new Etherpad-Go database. The new Etherpad database is already specified as expected in the settings.json file or environment variables.

## Plugins

Etherpad-Go supports plugins, but the plugin system is not yet fully fleshed out. Currently, only a few plugins are supported. You can find the list of supported plugins in the [plugins directory](./plugins).
You can activate them in the settings.json file or via environment variables.

E.g.

```json`
{
  "plugins": {
    "ep_align":{ "enabled": true }
  }
}
```

or ETHERPAD_PLUGINS_EP_ALIGN_ENABLED=true




