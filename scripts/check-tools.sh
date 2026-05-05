#!/usr/bin/env bash
# Verify the toolchain expected by this repo. Exits non-zero if any
# required tool is missing or below its minimum version.
set -u

GO_MIN=1.25.0
PROTOC_GEN_GO_MIN=1.36.0
PROTOC_GEN_GO_GRPC_MIN=1.5.0

red() { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[33m%s\033[0m\n' "$*"; }

# version_ge A B -> 0 if A >= B
version_ge() {
    [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]
}

fail=0

check_min() {
    local name=$1 path=$2 actual=$3 min=$4
    if version_ge "$actual" "$min"; then
        green  "  ok   $name $actual ($path)"
    else
        red    "  FAIL $name $actual < $min ($path)"
        fail=1
    fi
}

# --- go
if ! command -v go >/dev/null; then
    red "  FAIL go: not found in PATH"
    fail=1
else
    go_ver=$(go env GOVERSION | sed 's/^go//')
    check_min "go" "$(command -v go)" "$go_ver" "$GO_MIN"
fi

# --- protoc
if ! command -v protoc >/dev/null; then
    red "  FAIL protoc: not found in PATH"
    fail=1
else
    protoc_ver=$(protoc --version | awk '{print $NF}')
    green "  ok   protoc $protoc_ver ($(command -v protoc))"
fi

# --- protoc-gen-go (must resolve to the binary protoc itself would invoke)
if ! command -v protoc-gen-go >/dev/null; then
    red "  FAIL protoc-gen-go: not found in PATH"
    red "       install: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    fail=1
else
    pgg_path=$(command -v protoc-gen-go)
    pgg_ver=$(protoc-gen-go --version 2>&1 | awk '{print $NF}' | sed 's/^v//')
    check_min "protoc-gen-go" "$pgg_path" "$pgg_ver" "$PROTOC_GEN_GO_MIN"
    if [ "$fail" = "0" ] && [ -d "$(go env GOPATH)/bin" ]; then
        gopath_pgg="$(go env GOPATH)/bin/protoc-gen-go"
        if [ -x "$gopath_pgg" ] && [ "$gopath_pgg" != "$pgg_path" ]; then
            yellow "  warn another protoc-gen-go exists at $gopath_pgg"
            yellow "       protoc resolves via PATH; reorder PATH if you want that one"
        fi
    fi
fi

# --- protoc-gen-go-grpc (only needed for example regeneration)
if ! command -v protoc-gen-go-grpc >/dev/null; then
    yellow "  warn protoc-gen-go-grpc: not found (only needed to regen examples)"
    yellow "       install: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
else
    grpc_ver=$(protoc-gen-go-grpc --version 2>&1 | awk '{print $NF}' | sed 's/^v//')
    check_min "protoc-gen-go-grpc" "$(command -v protoc-gen-go-grpc)" "$grpc_ver" "$PROTOC_GEN_GO_GRPC_MIN"
fi

# --- PROTOPATH (where google/protobuf/*.proto live)
candidates=(
    "${PROTOPATH:-}"
    "/usr/local/include"
    "/usr/include"
    "$(command -v brew >/dev/null && brew --prefix protobuf 2>/dev/null)/include"
)
resolved=
for d in "${candidates[@]}"; do
    [ -z "$d" ] && continue
    if [ -f "$d/google/protobuf/descriptor.proto" ]; then
        resolved=$d
        break
    fi
done
if [ -n "$resolved" ]; then
    green "  ok   PROTOPATH $resolved"
    if [ "$resolved" != "/usr/local/include" ]; then
        yellow "       Makefile defaults to /usr/local/include on Linux."
        yellow "       Run: make -e PROTOPATH=$resolved <target>"
    fi
else
    red "  FAIL google/protobuf/descriptor.proto not found in any of: ${candidates[*]}"
    red "       install protobuf headers (Debian: apt install libprotobuf-dev)"
    fail=1
fi

if [ "$fail" -ne 0 ]; then
    echo
    red "toolchain check failed"
    exit 1
fi
echo
green "toolchain ok"
