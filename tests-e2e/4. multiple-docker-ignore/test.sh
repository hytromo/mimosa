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

CMD="mimosa remember -- docker buildx build --file Dockerfile.custom --build-arg DATE=${DATE} --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG} ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: false" "$TMP_FILE"

echo $RANDOM > ignored-files/random

# the real docker ignore does not ignore the ignored-files
CMD="mimosa remember -- docker buildx build --file Dockerfile.custom --build-arg DATE=${DATE} --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG}-v2 ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: false" "$TMP_FILE"

echo 'ignored-files/**' > Dockerfile.custom.dockerignore

CMD="mimosa remember -- docker buildx build --file Dockerfile.custom --build-arg DATE=${DATE} --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG}-v2 ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: false" "$TMP_FILE"

echo $RANDOM > ignored-files/random

CMD="mimosa remember -- docker buildx build --file Dockerfile.custom --build-arg DATE=${DATE} --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG}-v2 ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: true" "$TMP_FILE"

rm "$TMP_FILE"