#!/usr/bin/env bash

set -euxo pipefail

TMP_FILE=$(mktemp)
DATE=$(date +%s)

# Verify that a multi-platform image tag has non-empty platform os and architecture
assert_platforms() {
  local tag="$1"
  local manifest_json
  manifest_json=$(docker buildx imagetools inspect --raw "$tag")

  echo "$manifest_json" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for m in data.get('manifests', []):
    p = m.get('platform', {})
    os_val = p.get('os', '')
    arch_val = p.get('architecture', '')
    if not os_val or not arch_val:
        print(f'Empty platform found: os={os_val!r} architecture={arch_val!r}')
        sys.exit(1)
print('All platforms have os and architecture')
"
  echo "👍 $tag: platform info preserved"
}

echo
echo ">>> TEST 5.1: Normal build (cache miss) <<<"
echo
CMD="mimosa remember -- docker buildx build --build-arg DATE=${DATE} --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG} ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: false" "$TMP_FILE"
assert_platforms "${FULL_IMAGE_TAG}"

echo
echo ">>> TEST 5.2: retag-only with same content (cache hit) <<<"
echo
# Same build args as 5.1, new tag: retag-only should retag and output cache hit
CMD="mimosa remember --retag-only -- docker buildx build --build-arg DATE=${DATE} --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG}-retag ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: true" "$TMP_FILE"
assert_platforms "${FULL_IMAGE_TAG}-retag"

echo
echo ">>> TEST 5.3: retag-only with cache miss (no build, exit 0) <<<"
echo
# Different DATE => different hash => cache miss; retag-only must exit 0 and output false without building
DATE_OTHER=$((DATE + 10000))
CMD="mimosa remember --retag-only -- docker buildx build --build-arg DATE=${DATE_OTHER} --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG}-never-built ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: false" "$TMP_FILE"
# Tag must not exist (we never built)
if docker buildx imagetools inspect "${FULL_IMAGE_TAG}-never-built" 2>/dev/null; then
  echo "ERROR: ${FULL_IMAGE_TAG}-never-built should not exist after retag-only cache miss"
  exit 1
fi
echo "👍 retag-only cache miss exited 0 and did not push the tag"

rm "$TMP_FILE"
