#!/usr/bin/env bash
# End-to-end test of the Kubernetes deployment in doc/cluster.md, on the
# cluster kubectl points at (CI: kind), with the image etherpad-go:k8s-test
# already loaded into it.
#
#   test/kubernetes/run.sh <etherpad-go binary for the benches>
#
# 1. Three replicas and PostgreSQL come up; every pod sees the two others.
# 2. clusterbench writes through two pods: nothing lost, duplicated, diverged.
# 3. Optional (RUN_PLAYWRIGHT=1): the two-instance browser specs against two
#    different pods.
# 4. killbench: a pod deleted the normal way (SIGTERM, drain) while authors
#    type through it, then one deleted with --grace-period=0 --force: the
#    authors carry on through another pod, nothing lost or duplicated.
# 5. A rolling restart keeps the pad text, and the new pods form a cluster.
set -euo pipefail
bin=$(realpath "$1")
here=$(cd "$(dirname "$0")" && pwd)
logs=${K8S_TEST_LOGS:-/tmp/k8s-test}
mkdir -p "$logs"
pids=()
cleanup() {
  for p in "${pids[@]}"; do kill "$p" 2>/dev/null || true; done
}
trap cleanup EXIT

step() { echo; echo "=== $*"; }
fail() {
  echo "FAIL: $*" >&2
  kubectl get pods -o wide >&2 || true
  for p in $(kubectl get pods -l app=etherpad -o name 2>/dev/null); do
    echo "--- $p" >&2
    kubectl logs "$p" --tail=80 >&2 || true
  done
  exit 1
}

# pods prints the names of the etherpad pods that are running and not being
# deleted.
pods() {
  kubectl get pods -l app=etherpad \
    -o jsonpath='{range .items[*]}{.metadata.name} {.status.phase} {.metadata.deletionTimestamp}{"\n"}{end}' \
    | awk '$2 == "Running" && NF == 2 { print $1 }'
}

wait_ready() {
  kubectl rollout status deployment/etherpad --timeout=300s || fail "rollout"
  for _ in $(seq 1 60); do
    [ "$(pods | wc -l)" -eq 3 ] && return 0
    sleep 2
  done
  fail "expected 3 running pods"
}

# forward pod port: a port-forward in the background, waits until it answers.
forward() {
  kubectl port-forward "pod/$1" "$2:9001" > "$logs/pf-$2.log" 2>&1 &
  pids+=($!)
  for _ in $(seq 1 60); do
    curl -fs -o /dev/null "http://127.0.0.1:$2/health/ready" && return 0
    sleep 0.5
  done
  fail "port-forward to $1 on $2"
}

# joined pod: every pod has logged the two others joining.
check_cluster() {
  for _ in $(seq 1 60); do
    local ok=1
    for p in $(pods); do
      n=$(kubectl logs "$p" | grep "cluster node joined" | sed -E 's/.*"node": "([^"]+)".*/\1/' | sort -u | wc -l)
      [ "$n" -ge 2 ] || ok=0
    done
    [ $ok -eq 1 ] && return 0
    sleep 2
  done
  fail "the pods did not all see each other"
}

step "deploy"
kubectl apply -f "$here/manifests.yaml"
kubectl rollout status deployment/etherpad-db --timeout=300s || fail "database"
wait_ready
check_cluster
echo "3 pods, each sees the two others"

step "clusterbench through two pods"
read -r a b _ <<< "$(pods | tr '\n' ' ')"
forward "$a" 19001
forward "$b" 19002
"$bin" clusterbench -hosts http://127.0.0.1:19001,http://127.0.0.1:19002 \
  -pad k8s-persist -authors 8 -duration 20s 2>&1 | tee "$logs/clusterbench.log" | grep -vE "\s(INFO|DEBUG)\s"
grep -q '^PASS' "$logs/clusterbench.log" || fail "clusterbench"

if [ "${RUN_PLAYWRIGHT:-}" = 1 ]; then
  step "browser specs against two pods"
  (cd "$here/../../playwright" && EP_NODE_A=http://127.0.0.1:19001 EP_NODE_B=http://127.0.0.1:19002 \
    npx playwright test -c cluster.config.ts) || fail "playwright"
fi
curl -fs "http://127.0.0.1:19001/p/k8s-persist/export/txt" > "$logs/before.txt" || fail "export"
[ -s "$logs/before.txt" ] || fail "empty pad after clusterbench"
cleanup; pids=()

killbench() { # victim survivor kill-command
  forward "$1" 19011
  forward "$2" 19012
  "$bin" killbench -victim http://127.0.0.1:19011 -survivors http://127.0.0.1:19012 \
    -kill-cmd "$3" -authors 8 -duration 20s -kill-after 6s 2>&1 | tee "$logs/killbench.log" | grep -vE "\s(INFO|DEBUG)\s"
  grep -q '^PASS' "$logs/killbench.log" || fail "killbench ($3)"
  cleanup; pids=()
}

step "killbench: pod deleted (SIGTERM, drain)"
read -r a b _ <<< "$(pods | tr '\n' ' ')"
killbench "$a" "$b" "kubectl delete pod $a --wait=false"
kubectl wait --for=delete "pod/$a" --timeout=120s || fail "pod $a did not go"
wait_ready

step "killbench: pod deleted at once (--grace-period=0 --force)"
read -r a b _ <<< "$(pods | tr '\n' ' ')"
killbench "$a" "$b" "kubectl delete pod $a --grace-period=0 --force --wait=false"
kubectl wait --for=delete "pod/$a" --timeout=120s || fail "pod $a did not go"
wait_ready

step "rolling restart"
kubectl rollout restart deployment/etherpad
wait_ready
check_cluster
read -r a _ <<< "$(pods | tr '\n' ' ')"
forward "$a" 19021
curl -fs "http://127.0.0.1:19021/p/k8s-persist/export/txt" > "$logs/after.txt" || fail "export after the restart"
cmp -s "$logs/before.txt" "$logs/after.txt" || fail "the pad changed across the rolling restart"
echo "pad text unchanged across the restart, new pods form a cluster"

echo
echo "PASS"
