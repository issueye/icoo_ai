#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PROJECT_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
CLEAN=0
RUN_TESTS=0
NO_COLOUR=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --clean)
      CLEAN=1
      ;;
    --run-tests)
      RUN_TESTS=1
      ;;
    --no-colour|--no-color)
      NO_COLOUR=1
      ;;
    -h|--help)
      echo "Usage: scripts/build.sh [--clean] [--run-tests] [--no-colour]"
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Required command '$1' was not found on PATH." >&2
    exit 1
  fi
}

cd "$PROJECT_ROOT"

require_command wails3
require_command npm

if [ "$CLEAN" -eq 1 ]; then
  rm -rf frontend/dist bin
fi

if [ "$RUN_TESTS" -eq 1 ]; then
  require_command go
  go test ./...
fi

if [ "$NO_COLOUR" -eq 1 ]; then
  wails3 build -nocolour
else
  wails3 build
fi

if [ -f "$PROJECT_ROOT/bin/agent_chat.exe" ]; then
  echo "Build complete: $PROJECT_ROOT/bin/agent_chat.exe"
else
  echo "Build complete."
fi
