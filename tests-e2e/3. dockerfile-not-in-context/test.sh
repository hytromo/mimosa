#!/usr/bin/env bash

set -euxo pipefail

TMP_FILE=$(mktemp)
DATE=$(date +%s)

cd ignored-files

echo
echo ">>> TEST 3.1 <<<"
echo
CMD="mimosa remember -- docker buildx build --build-arg DATE=${DATE} --file ../Dockerfile --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG} ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: false" "$TMP_FILE"

echo
echo ">>> TEST 3.2 <<<"
echo
echo $RANDOM > random

CMD="mimosa remember -- docker buildx build --build-arg DATE=${DATE} --file ../Dockerfile --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG}-v2 ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: true" "$TMP_FILE"

rm "$TMP_FILE"