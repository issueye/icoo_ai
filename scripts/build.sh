#!/usr/bin/env sh
set -eu

VERSION=""
OUT_DIR="dist"
TARGET_GOOS=""
TARGET_GOARCH=""
ALL=0
SKIP_TESTS=0
CLEAN=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      VERSION="$2"
      shift 2
      ;;
    --out-dir)
      OUT_DIR="$2"
      shift 2
      ;;
    --goos)
      TARGET_GOOS="$2"
      shift 2
      ;;
    --goarch)
      TARGET_GOARCH="$2"
      shift 2
      ;;
    --all)
      ALL=1
      shift
      ;;
    --skip-tests)
      SKIP_TESTS=1
      shift
      ;;
    --clean)
      CLEAN=1
      shift
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
cd "$REPO_ROOT"

if [ "$CLEAN" -eq 1 ] && [ -d "$OUT_DIR" ]; then
  rm -rf "$OUT_DIR"
fi
mkdir -p "$OUT_DIR"
if [ -f "$REPO_ROOT/configs/config.example.toml" ]; then
  cp "$REPO_ROOT/configs/config.example.toml" "$OUT_DIR/config.example.toml"
fi

if [ -z "$VERSION" ]; then
  VERSION=$(git describe --tags --always --dirty 2>/dev/null || true)
  if [ -z "$VERSION" ]; then
    VERSION="dev"
  fi
fi

COMMIT=$(git rev-parse --short HEAD 2>/dev/null || true)
if [ -z "$COMMIT" ]; then
  COMMIT="unknown"
fi
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS="-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$BUILD_DATE"

if [ "$SKIP_TESTS" -eq 0 ]; then
  go test ./... -count=1
fi

build_target() {
  target_goos="$1"
  target_goarch="$2"
  binary="icoo-ai-$target_goos-$target_goarch"
  if [ "$target_goos" = "windows" ]; then
    binary="$binary.exe"
  fi
  out_path="$OUT_DIR/$binary"
  echo "building $out_path"
  GOOS="$target_goos" GOARCH="$target_goarch" go build -trimpath -ldflags "$LDFLAGS" -o "$out_path" ./cmd/icoo-ai
}

if [ "$ALL" -eq 1 ]; then
  build_target windows amd64
  build_target windows arm64
  build_target linux amd64
  build_target linux arm64
  build_target darwin amd64
  build_target darwin arm64
else
  if [ -z "$TARGET_GOOS" ]; then
    TARGET_GOOS=$(go env GOOS)
  fi
  if [ -z "$TARGET_GOARCH" ]; then
    TARGET_GOARCH=$(go env GOARCH)
  fi
  build_target "$TARGET_GOOS" "$TARGET_GOARCH"
fi

echo "build artifacts written to $OUT_DIR"
