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

cd ignored-files

echo
echo ">>> TEST 3.1 <<<"
echo
CMD="mimosa remember -- docker buildx build --build-arg DATE=${DATE} --file ../Dockerfile --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG} ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: false" "$TMP_FILE"
assert_platforms "${FULL_IMAGE_TAG}"

echo
echo ">>> TEST 3.2 <<<"
echo
echo $RANDOM > random

CMD="mimosa remember -- docker buildx build --build-arg DATE=${DATE} --file ../Dockerfile --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG}-v2 ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: true" "$TMP_FILE"
assert_platforms "${FULL_IMAGE_TAG}-v2"

rm "$TMP_FILE"