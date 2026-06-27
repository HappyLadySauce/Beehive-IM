#!/usr/bin/env bash
# Beehive-IM RPC code generation entry (Unix shell)
# Beehive-IM RPC 代码生成入口（Unix shell）
#
# Scans proto/*.proto and runs goctl inside each matching services/{name}/ directory.
# 扫描 proto/*.proto，在对应的 services/{name}/ 目录内执行 goctl 生成 zRPC 代码。
#
# goctl is run inside services/{name}/ with:
#   --proto_path=../..  (protoc include path for repository root)
#   --go_out=. --go-grpc_out=.  (paired with go_package = "./pb" in proto files)
#
# Usage / 用法:
#   ./scripts/generate.sh
#   ./scripts/generate.sh auth
#   SERVICE=user ./scripts/generate.sh

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_ROOT="${ROOT}/proto"
SERVICE="${SERVICE:-${1:-}}"

if ! command -v goctl >/dev/null 2>&1; then
  echo "goctl is not installed or not in PATH. Install: go install github.com/zeromicro/go-zero/tools/goctl@latest" >&2
  exit 1
fi

if [[ ! -d "$PROTO_ROOT" ]]; then
  echo "Proto directory not found: $PROTO_ROOT" >&2
  exit 1
fi

shopt -s nullglob
proto_files=("$PROTO_ROOT"/*.proto)
shopt -u nullglob

if [[ ${#proto_files[@]} -eq 0 ]]; then
  echo "No .proto files found in $PROTO_ROOT" >&2
  exit 1
fi

matched=()
for proto in "${proto_files[@]}"; do
  name="$(basename "$proto" .proto)"
  if [[ -n "$SERVICE" && "$name" != "$SERVICE" ]]; then
    continue
  fi
  matched+=("$proto")
done

if [[ ${#matched[@]} -eq 0 ]]; then
  echo "No proto file matched service: $SERVICE" >&2
  exit 1
fi

for proto in "${matched[@]}"; do
  name="$(basename "$proto" .proto)"
  svc_dir="${ROOT}/services/${name}"
  proto_rel="../../proto/${name}.proto"

  mkdir -p "$svc_dir"
  echo "Generating services/${name} from proto/$(basename "$proto") ..."

  (
    cd "$svc_dir"
    goctl rpc protoc "$proto_rel" \
      --proto_path=../.. \
      --go_out=. \
      --go-grpc_out=. \
      --zrpc_out=. \
      --client=true
  )

  echo "Done: services/${name}"
done
