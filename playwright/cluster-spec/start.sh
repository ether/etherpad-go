#!/usr/bin/env bash
# Starts two etherpad-go instances (ports 9001 and 9002) as one cluster on a
# PostgreSQL database and waits until both answer and see each other.
# Usage: start.sh <etherpad-go binary> <log dir>
# Database: EP_PG_HOST/PORT/DB/USER/PASSWORD (defaults: 127.0.0.1, 5432,
# etherpad_cluster, etherpad, etherpad).
set -euo pipefail
bin=$1
logs=$2
mkdir -p "$logs"
for n in 1 2; do
  ETHERPAD_PORT=900$n \
  ETHERPAD_DBTYPE=postgres \
  ETHERPAD_DBSETTINGS_HOST=${EP_PG_HOST:-127.0.0.1} \
  ETHERPAD_DBSETTINGS_PORT=${EP_PG_PORT:-5432} \
  ETHERPAD_DBSETTINGS_DATABASE=${EP_PG_DB:-etherpad_cluster} \
  ETHERPAD_DBSETTINGS_USER=${EP_PG_USER:-etherpad} \
  ETHERPAD_DBSETTINGS_PASSWORD=${EP_PG_PASSWORD:-etherpad} \
  ETHERPAD_CLUSTER_MODE=on \
  ETHERPAD_CLUSTER_BINDADDR=127.0.0.1 \
  ETHERPAD_CLUSTER_ADVERTISEADDR=127.0.0.1 \
  ETHERPAD_CLUSTER_BINDPORT=$((7945 + n)) \
  nohup "$bin" > "$logs/node$n.log" 2>&1 &
  echo $! > "$logs/node$n.pid"
  # The first instance creates the schema; start the second after it.
  for _ in $(seq 1 120); do
    curl -fs -o /dev/null "http://localhost:900$n/" && break
    sleep 0.5
  done
  curl -fs -o /dev/null "http://localhost:900$n/" || { cat "$logs/node$n.log"; exit 1; }
done
for _ in $(seq 1 60); do
  grep -q "cluster node joined" "$logs/node2.log" && exit 0
  sleep 0.5
done
echo "the instances did not join each other" >&2
cat "$logs"/node*.log >&2
exit 1
