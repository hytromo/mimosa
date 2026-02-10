#!/usr/bin/env bash

set -euxo pipefail

# Save original contents
ORIGINAL_RANDOM_CONTENT=$(cat ignored-files/random 2>/dev/null || echo "")
ORIGINAL_DOCKERIGNORE_CONTENT=$(cat Dockerfile.custom.dockerignore 2>/dev/null || echo "")

cleanup() {
  echo "$ORIGINAL_RANDOM_CONTENT" > ignored-files/random
  echo "$ORIGINAL_DOCKERIGNORE_CONTENT" > Dockerfile.custom.dockerignore
}
trap cleanup EXIT

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
echo ">>> TEST 4.1 <<<"
echo

CMD="mimosa remember -- docker buildx build --file Dockerfile.custom --build-arg DATE=${DATE} --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG} ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: false" "$TMP_FILE"
assert_platforms "${FULL_IMAGE_TAG}"

echo
echo ">>> TEST 4.2 <<<"
echo

echo $RANDOM > ignored-files/random

# the real docker ignore does not ignore the ignored-files
CMD="mimosa remember -- docker buildx build --file Dockerfile.custom --build-arg DATE=${DATE} --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG}-v2 ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: false" "$TMP_FILE"
assert_platforms "${FULL_IMAGE_TAG}-v2"

echo
echo ">>> TEST 4.3 <<<"
echo

echo 'ignored-files/**' > Dockerfile.custom.dockerignore

CMD="mimosa remember -- docker buildx build --file Dockerfile.custom --build-arg DATE=${DATE} --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG}-v2 ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: false" "$TMP_FILE"
assert_platforms "${FULL_IMAGE_TAG}-v2"

echo
echo ">>> TEST 4.4 <<<"
echo
echo $RANDOM > ignored-files/random

CMD="mimosa remember -- docker buildx build --file Dockerfile.custom --build-arg DATE=${DATE} --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG}-v2 ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: true" "$TMP_FILE"
assert_platforms "${FULL_IMAGE_TAG}-v2"

rm "$TMP_FILE"