#!/usr/bin/env bash
# Replays every tool against a fake server, then checks each GraphQL document
# and its variables against Shopify's published Admin schema. Needs node + npm.
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${SHOPIFY_API_VERSION:-2026-07}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

( cd "$WORK" && npm init -y >/dev/null && npm i graphql >/dev/null 2>&1 )
cp scripts/validate.js "$WORK/validate.js"
( cd "$WORK" && node -e '
const {getIntrospectionQuery}=require("graphql");
require("fs").writeFileSync("q.json",JSON.stringify({query:getIntrospectionQuery()}));' )
curl -sf -X POST "https://shopify.dev/admin-graphql-direct-proxy/${VERSION}" \
  -H 'Content-Type: application/json' -d @"$WORK/q.json" -o "$WORK/schema.json"

SHOPIFY_API_VERSION="$VERSION" VALIDATE_OUT="$WORK/captured.json" go test -run TestToolsSendValidGraphQL ./... >/dev/null
( cd "$WORK" && node validate.js captured.json schema.json )
