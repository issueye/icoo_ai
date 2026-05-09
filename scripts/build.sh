#!/usr/bin/env sh
set -eu

TARGET="all"
CLEAN=0
SKIP_TESTS=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --target)
      TARGET="$2"
      shift 2
      ;;
    --clean)
      CLEAN=1
      shift
      ;;
    --skip-tests)
      SKIP_TESTS=1
      shift
      ;;
    -h|--help)
      echo "Usage: scripts/build.sh [--target all|chat|gateway|server] [--clean] [--skip-tests]"
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

run_step() {
  echo "==> $1"
  shift
  "$@"
}

build_agent_chat() {
  args=""
  [ "$CLEAN" -eq 1 ] && args="$args --clean"
  [ "$SKIP_TESTS" -eq 0 ] && args="$args --run-tests"
  # shellcheck disable=SC2086
  "$REPO_ROOT/agent_chat/scripts/build.sh" $args
}

build_agent_gateway() {
  cd "$REPO_ROOT/agent_gateway"
  if [ "$SKIP_TESTS" -eq 0 ]; then
    go test ./... -count=1
  fi
  if [ "$CLEAN" -eq 1 ]; then
    rm -rf dist
  fi
  mkdir -p dist
  out="dist/agent-gateway"
  if [ "$(go env GOOS)" = "windows" ]; then
    out="${out}.exe"
  fi
  go build -trimpath -o "$out" ./cmd/agent-gateway
  echo "Build complete: $REPO_ROOT/agent_gateway/$out"
}

build_agent_server() {
  args=""
  [ "$CLEAN" -eq 1 ] && args="$args --clean"
  [ "$SKIP_TESTS" -eq 1 ] && args="$args --skip-tests"
  # shellcheck disable=SC2086
  "$REPO_ROOT/agent_server/scripts/build.sh" $args
}

case "$TARGET" in
  all)
    run_step "Build agent_chat" build_agent_chat
    run_step "Build agent_gateway" build_agent_gateway
    run_step "Build agent_server" build_agent_server
    ;;
  chat)
    run_step "Build agent_chat" build_agent_chat
    ;;
  gateway)
    run_step "Build agent_gateway" build_agent_gateway
    ;;
  server)
    run_step "Build agent_server" build_agent_server
    ;;
  *)
    echo "Invalid target: $TARGET (expected all|chat|gateway|server)" >&2
    exit 2
    ;;
esac
