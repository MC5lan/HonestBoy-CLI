#!/usr/bin/env bash
set -euo pipefail
fail=0
check() {
  local pkg="$1"; shift
  for dep in "$@"; do
    if go list -deps "$pkg" 2>/dev/null | grep -q "^$dep$"; then
      echo "VIOLATION: $pkg depends on $dep"; fail=1
    fi
  done
}
check ./internal/core ./internal/tui ./internal/llm ./internal/tools ./internal/store ./internal/engine
check ./internal/llm  ./internal/tui ./internal/cli
check ./internal/tools ./internal/tui ./internal/cli
check ./internal/tui  ./internal/llm ./internal/tools ./internal/store
check ./internal/cli  ./internal/tui
exit $fail