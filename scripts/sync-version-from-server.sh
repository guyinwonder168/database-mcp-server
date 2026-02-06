#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

SERVER_FILE="internal/mcp/server.go"
README_FILE="README.md"
OPENAPI_FILE="docs/mcp-openapi.yaml"

VERSION="$(awk -F'"' '/^const MCPVersion = / {print $2; exit}' "$SERVER_FILE")"
if [ -z "$VERSION" ]; then
  echo "Failed to read MCPVersion from $SERVER_FILE"
  exit 1
fi

if ! echo "$VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "Invalid MCPVersion format in $SERVER_FILE: $VERSION"
  exit 1
fi

VERSION_NO_V="${VERSION#v}"

# Keep README version badge/release link/image tags aligned with MCPVersion.
sed -E -i "s#(img.shields.io/badge/Version-)v[0-9]+\.[0-9]+\.[0-9]+(-blue\\.svg)#\\1${VERSION}\\2#g" "$README_FILE"
sed -E -i "s#(releases/tag/)v[0-9]+\.[0-9]+\.[0-9]+#\\1${VERSION}#g" "$README_FILE"
sed -E -i "s#(ghcr.io/guyinwonder168/database-mcp-server:)v[0-9]+\.[0-9]+\.[0-9]+#\\1${VERSION}#g" "$README_FILE"
sed -E -i "s#(\\*\\*Version:\\*\\* )v[0-9]+\\.[0-9]+\\.[0-9]+#\\1${VERSION}#g" "$README_FILE"

# Keep OpenAPI info.version (without leading v) aligned.
sed -E -i "0,/version: \"[0-9]+\.[0-9]+\.[0-9]+\"/s//version: \"${VERSION_NO_V}\"/" "$OPENAPI_FILE"

echo "Synced version metadata from MCPVersion=${VERSION}"
