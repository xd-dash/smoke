#!/usr/bin/env bash
set -Eeuo pipefail
exec bash "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/host-operator.sh" auth-refresh
