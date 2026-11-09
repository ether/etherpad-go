# Clustering (several instances, one database)

Etherpad-Go can run as several instances on one PostgreSQL or MySQL database
behind a plain, **stateless** load balancer or Kubernetes Ingress — no sticky
sessions, no Redis, no extra service. Any instance can serve any pad; when a pod
dies, its users reconnect to another pod and carry on.

In Kubernetes with `dbType` `postgres` or `mysql` this is **on by default**
(elsewhere set `cluster.mode` to `on`). All an installation needs is that the
instances can reach each other on port `7946` (TCP and UDP).

## Guarantees

* **No accepted change is ever lost.** A client only gets `ACCEPT_COMMIT` after
  its revision is stored in the database. Two instances can never store the same
  revision number: the database rejects the second writer, which rebases the
  change onto the newer revisions and stores it again.
* **Every instance converges to the same text.** The stored revisions are the
  truth; an instance's in-memory copy is only a cache and is refreshed from the
  database before every write.
* **Live updates across instances.** Users on different pods see each other's
  edits, cursors/presence, chat and user-info changes within milliseconds.
* **Losing the gossip bus degrades speed, not correctness.** If the instances
  cannot talk to each other, pads are still correct — instances notice newer
  revisions in the database when they write or, for pads with connected users,
  every 2 seconds.

## How it works

```
             stateless Ingress / LB  (any request → any pod)
               │             │             │
          ┌────▼───┐    ┌────▼───┐    ┌────▼───┐
          │ pod A  │◄──►│ pod B  │◄──►│ pod C  │   memberlist gossip, :7946 TCP+UDP
          └────┬───┘    └────┬───┘    └────┬───┘   (hints + live events, encrypted)
               │             │             │
          ┌────▼─────────────▼─────────────▼───┐
          │   PostgreSQL / MySQL  (the truth)  │   revisions, chat, sessions,
          │                                    │   per-pad lock, cluster_nodes
          └────────────────────────────────────┘
```

1. **Database as the single source of truth.** Revisions (`padRev`, primary key
   pad + revision number) and chat messages are written with a plain `INSERT`;
   a duplicate key is a conflict (`db.ErrConflict`), never an overwrite. The
   in-memory pad changes only after the insert succeeded.
2. **Per-pad database lock.** Writes to one pad take a lock in the database
   (`pg_advisory_lock` on PostgreSQL, `GET_LOCK` on MySQL), so instances do not
   race on the same pad. After taking the lock an instance loads the revisions it
   is missing (deltas only, not the whole text). Different pads never block each
   other.
3. **Gossip bus (HashiCorp memberlist).** When an instance stores a revision it
   tells the others ("pad X is at revision N") over UDP; they load the delta and
   push it to their clients. Chat, presence, join/leave, author changes, pad
   deletion and admin messages travel over TCP the same way. The bus is
   encrypted with a key that the first instance generates and stores in the
   database (`cluster_gossip_key`), so all instances on one database share it
   automatically.
4. **Discovery through the database.** Each instance writes its gossip address
   into the `cluster_nodes` table every 5 seconds (JDBC-ping style, like
   Keycloak/Infinispan). New instances join the addresses found there;
   entries older than 10 minutes are pruned. No Kubernetes API access, no DNS
   discovery and no seed list are needed.
5. **Large pads.** Only every 100th revision stores the full text (key
   revision); the others store just the changeset. Loading or catching up
   rebuilds from the nearest key revision, so a commit costs the same on a
   1 MB pad as on a small one.

### What happens when…

| Situation | Behaviour |
|-----------|-----------|
| Two users on different pods type into the same pad | Both changes are kept. The second writer gets a conflict, rebases and stores again (max. 5 retries), then the client gets `ACCEPT_COMMIT`. |
| A pod is killed while users type | Their WebSocket breaks and the client reconnects through the Ingress to another pod, telling it the last revision it has. That pod sends the missed revisions — including the client's own in-flight change if it had been stored, which the client then treats as acknowledged. Changes not yet stored stay in the editor and are sent again. Acknowledged changes are always in the database. |
| A new pod starts | It runs migrations under a database lock (safe when pods start at once), registers itself in `cluster_nodes` and joins the others. |
| Gossip port blocked | Everything stays correct; cross-pod updates arrive within ~2 s (catch-up loop) instead of milliseconds. A warning is logged. |
| Database unreachable | Writes fail and clients are not acknowledged — nothing is silently dropped. A pad that cannot be locked or brought up to date is not served from a stale copy: HTTP calls get `503`, pad sockets are closed so the client reconnects and resends. |
| A pod is shut down (SIGTERM, rollout) | It reports not ready and refuses new pad connections, waits `shutdownDelaySeconds` for the load balancer, gives up its leader locks, then closes the pad connections so clients reconnect to another pod. Changes in progress are stored first. |
| The same user opens a pad through two pods | As on one instance: the older session is disconnected ("userdup"). |
| The pod running a background job dies | Its leader lock is tied to its database connection and released with it; another pod takes over within 5 seconds. |

## Settings

Under the `cluster` key in `settings.json`, or as environment variables
(`ETHERPAD_` + key path with `_`, case-insensitive):

| Key | Env variable | Default | Meaning |
|-----|--------------|---------|---------|
| `cluster.mode` | `ETHERPAD_CLUSTER_MODE` | `auto` | `auto`: on for `postgres`/`mysql` when running in Kubernetes or with `POD_IP` set, off otherwise — an existing single instance does not open the gossip port after an upgrade. `on`: must start, otherwise the instance exits. `off`: single instance. |
| `cluster.bindAddr` | `ETHERPAD_CLUSTER_BINDADDR` | the advertise address, else all interfaces | Address the gossip listens on. |
| `cluster.bindPort` | `ETHERPAD_CLUSTER_BINDPORT` | `7946` | Port for gossip, **TCP and UDP**. |
| `cluster.advertiseAddr` | `ETHERPAD_CLUSTER_ADVERTISEADDR` | `$POD_IP`, else a private IP of the host | Address the other instances use to reach this one. |
| `cluster.advertisePort` | `ETHERPAD_CLUSTER_ADVERTISEPORT` | bind port | Port the others use, if it differs (NAT). |
| `cluster.shutdownDelaySeconds` | `ETHERPAD_CLUSTER_SHUTDOWNDELAYSECONDS` | `5` | How long a pod keeps serving after SIGTERM while reporting not ready, before it closes its connections. |

```json
"cluster": {
  "mode": "auto",
  "bindAddr": "",
  "bindPort": 7946,
  "advertiseAddr": "",
  "advertisePort": 0,
  "shutdownDelaySeconds": 5
}
```

With `mode: auto`, if the bus cannot start (e.g. port in use), the instance logs
a warning and runs without it; with `mode: on` it refuses to start.

**SQLite and the memory store** are for single instances. Running several
processes on one SQLite file is not supported.

## Kubernetes

Requirements:

* PostgreSQL or MySQL reachable from all pods (e.g. CloudNativePG, a managed DB).
* Pods can reach each other on `7946/TCP` and `7946/UDP` (default in most
  clusters; allow it explicitly if you use NetworkPolicies).
* `POD_IP` set through the downward API, so each pod advertises its own IP.
* Enough database connections: each pod uses up to 25 for its work plus up to
  14 for locks (pad locks, leader lock, migrations), so plan for about 40 per
  pod in `max_connections`.
* `terminationGracePeriodSeconds` longer than `cluster.shutdownDelaySeconds`
  plus a few seconds (the default 30 is fine).

No headless Service, RBAC or sticky sessions are needed — the Ingress can
balance freely.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: etherpad
spec:
  replicas: 3
  selector:
    matchLabels: { app: etherpad }
  template:
    metadata:
      labels: { app: etherpad }
    spec:
      terminationGracePeriodSeconds: 30
      containers:
        - name: etherpad
          image: ghcr.io/ether/etherpad-go:develop
          env:
            - name: POD_IP
              valueFrom:
                fieldRef: { fieldPath: status.podIP }
            - name: ETHERPAD_DBTYPE
              value: postgres
            - name: ETHERPAD_DBSETTINGS_HOST
              value: etherpad-db-rw
            - name: ETHERPAD_DBSETTINGS_PORT
              value: "5432"
            - name: ETHERPAD_DBSETTINGS_DATABASE
              value: etherpad
            - name: ETHERPAD_DBSETTINGS_USER
              valueFrom: { secretKeyRef: { name: etherpad-db, key: username } }
            - name: ETHERPAD_DBSETTINGS_PASSWORD
              valueFrom: { secretKeyRef: { name: etherpad-db, key: password } }
            # Updates are rolled out by Kubernetes, not by the pods themselves.
            - name: ETHERPAD_UPDATES_TIER
              value: notify
          ports:
            - { name: http,        containerPort: 9001, protocol: TCP }
            - { name: gossip-tcp,  containerPort: 7946, protocol: TCP }
            - { name: gossip-udp,  containerPort: 7946, protocol: UDP }
          # Ready: started, not shutting down, database reachable.
          readinessProbe:
            httpGet: { path: /health/ready, port: http }
            periodSeconds: 5
          # Live: the process answers. It does not check the database, so a
          # database outage does not get every pod restarted.
          livenessProbe:
            httpGet: { path: /health/live, port: http }
            periodSeconds: 10
            failureThreshold: 6
---
apiVersion: v1
kind: Service
metadata:
  name: etherpad
spec:
  selector: { app: etherpad }
  ports:
    - { name: http, port: 80, targetPort: http }
# The gossip port is pod-to-pod only and must not be exposed through the Service.
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: etherpad
spec:
  minAvailable: 1
  selector:
    matchLabels: { app: etherpad }
```

Ingress: any normal Ingress to the `etherpad` Service works. Make sure WebSocket
connections are allowed and idle timeouts are long (e.g. ingress-nginx:
`nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"` and
`proxy-send-timeout: "3600"`). No `affinity`/`session-cookie` annotations are
needed.

If you use NetworkPolicies, allow the pods to talk to each other on the gossip
port:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: etherpad-gossip
spec:
  podSelector:
    matchLabels: { app: etherpad }
  policyTypes: [Ingress]
  ingress:
    - from:
        - podSelector:
            matchLabels: { app: etherpad }
      ports:
        - { port: 7946, protocol: TCP }
        - { port: 7946, protocol: UDP }
```

### Rolling updates

Rolling updates work out of the box: migrations run under a database lock, so
pods starting at the same time do not race, and users on a terminated pod
reconnect to another one. During a rollout old and new pods briefly share the
database; check the release notes of versions with schema changes.

### Checking that it works

* The log of each pod shows the cluster members joining.
* Open the same pad twice in different browsers; with 2+ replicas the
  connections usually land on different pods, and edits appear live in both.
* `etherpad-go clusterbench -hosts http://pod-a:9001,http://pod-b:9001 -authors 8 -duration 30s`
  writes through several instances at once and reports commit latency,
  cross-pod latency and whether any accepted change was lost, duplicated or
  diverged (`lost: 0, duplicated: 0, divergent instances: []`). Add
  `-prefill 1000000` to test a 1 MB pad.

## What changes in a cluster

* **Background jobs run on one pod.** The pad retention sweep runs on the
  elected leader only (a database lock; `/health` shows `retentionLeader`).
  Opening an expired pad still expires it on any pod.
* **The self-updater only notifies.** Tiers above `notify` are treated as
  `notify`; update by rolling out a new image.
* **Settings are read-only in the admin UI.** A change there would only reach
  one pod and be lost with it. Configure through `settings.json` or the
  environment of all pods (ConfigMap) and roll them out.
* **Pad metadata** (saved revisions, public status, read-only id) is reloaded
  under the pad lock before it is changed and written column by column, so
  changes through different pods do not overwrite each other. Every pod hands
  out the same read-only link.

## Health endpoints

| Endpoint | Use | Answers |
|----------|-----|---------|
| `/health/live` | liveness probe | `200` while the process serves requests |
| `/health/ready` | readiness probe | `200` when started, not shutting down and the database is reachable; `503` otherwise |
| `/health` | monitoring | database, sessions and a `cluster` check: `enabled`, `members` (instances this pod sees) and `retentionLeader`. A clustered database without a running bus is a warning, not a failure. |

The probes need no login, also when the pads require authentication.

## Testing failover

`etherpad-go killbench` writes through one instance, kills it with SIGKILL
mid-typing and checks that the authors carry on through the others without
losing or duplicating a change:

```
etherpad-go killbench -victim http://127.0.0.1:9101 -survivors http://127.0.0.1:9102 \
  -victim-pid <pid of the first instance> -authors 8 -duration 15s -kill-after 5s
```

It runs against local processes (it kills by process id). On a test machine
with PostgreSQL, 8 authors and about 1,150 changes per run, nothing was lost
or duplicated and the longest wait for an acknowledgement across the kill was
about 0.3-0.4 s.

## Current limitations

* **User counts** in the admin statistics are per pod.
* **Sheets** (the spreadsheet document type) are not cluster-aware yet: their
  live updates are only sent to users on the same pod, and their storage has
  not been checked for concurrent writers on different pods. Use pads only,
  or one replica, if you rely on sheets.
* **Database connections.** The pool sizes are not configurable yet (25 work
  connections plus up to 14 for locks per pod).
* **Mixed versions during a rollout.** Old and new pods briefly share the
  database; check the release notes of versions with schema changes.
