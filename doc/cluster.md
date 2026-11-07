# Clustering (several instances, one database)

Etherpad-Go can run as several instances on one PostgreSQL or MySQL database
behind a plain, **stateless** load balancer or Kubernetes Ingress — no sticky
sessions, no Redis, no extra service. Any instance can serve any pad; when a pod
dies, its users reconnect to another pod and carry on.

With `dbType` `postgres` or `mysql` this is **on by default**. All an
installation needs is that the instances can reach each other on port `7946`
(TCP and UDP).

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
| Database unreachable | Writes fail and clients are not acknowledged — nothing is silently dropped. |

## Settings

Under the `cluster` key in `settings.json`, or as environment variables
(`ETHERPAD_` + key path with `_`, case-insensitive):

| Key | Env variable | Default | Meaning |
|-----|--------------|---------|---------|
| `cluster.mode` | `ETHERPAD_CLUSTER_MODE` | `auto` | `auto`: on for `postgres`/`mysql`, off otherwise. `on`: must start, otherwise the instance exits. `off`: single instance. |
| `cluster.bindAddr` | `ETHERPAD_CLUSTER_BINDADDR` | `0.0.0.0` | Address the gossip listens on. |
| `cluster.bindPort` | `ETHERPAD_CLUSTER_BINDPORT` | `7946` | Port for gossip, **TCP and UDP**. |
| `cluster.advertiseAddr` | `ETHERPAD_CLUSTER_ADVERTISEADDR` | `$POD_IP`, else a private IP of the host | Address the other instances use to reach this one. |
| `cluster.advertisePort` | `ETHERPAD_CLUSTER_ADVERTISEPORT` | bind port | Port the others use, if it differs (NAT). |

```json
"cluster": {
  "mode": "auto",
  "bindAddr": "0.0.0.0",
  "bindPort": 7946,
  "advertiseAddr": "",
  "advertisePort": 0
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
          readinessProbe:
            httpGet: { path: /health, port: http }
            periodSeconds: 5
          livenessProbe:
            httpGet: { path: /health, port: http }
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

## Current limitations

These are known and planned, not bugs in the guarantees above:

* **Background jobs run on every pod.** Pad retention/cleanup runs on each
  instance. The jobs are idempotent, but they do redundant work; leader election
  is planned.
* **Self-updater.** Keep `updates.tier` at `notify` or `off` in a cluster —
  pods must be updated by rolling out a new image, not by replacing their own
  binary.
* **Settings changed in the admin UI** apply to the pod that handled the
  request only. In a cluster, configure through `settings.json`/env (ConfigMap).
* **Pad metadata** (saved revisions, public status, read-only id) is stored as
  part of the pad row and last-writer-wins between pods. Text and chat are not
  affected.
* **`/health`** checks the database and the session store; it does not yet
  report cluster membership, and there is no separate readiness endpoint or
  connection draining on shutdown yet.
* **User counts** in the admin statistics are per pod.
* **Kill test.** Killing a pod mid-typing is covered by the design above, but
  not yet by an automated test.
