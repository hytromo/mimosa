# GitHub Actions

You can find full workflows in the [example-github-action](./example-github-action.yml) workflow. See below for the important bits.

## Recommended way: build-push-action

The easiest way to use Mimosa is with the all-in-one `build-push-action`. It handles both Mimosa's setup and building and has compatible inputs with the official `docker/build-push-action`.

```yaml
- uses: hytromo/mimosa/gh/build-push-action@v6-build-push
  with:
    # All docker/build-push-action inputs are supported
    platforms: linux/amd64,linux/arm64
    push: true
    tags: my-org/my-image:${{ github.sha }}
    context: .
```

This action wraps `docker/build-push-action` with Mimosa integration, so all the [standard inputs](https://github.com/docker/build-push-action#inputs) are supported. The tag of this action is following the official `docker/build-push-action` tag.

### Additional inputs

| Input | Description | Default |
|-------|-------------|---------|
| `mimosa-setup-enabled` | Enable/disable mimosa setup | `true` |
| `mimosa-setup-version` | Mimosa binary version | `latest` |
| `mimosa-setup-tools-file` | Path to .tool-versions file | `.tool-versions` |
| `mimosa-retag-only` | If true, only check cache and retag on hit; do not run Docker/Buildx or build on miss. Use in a workflow to run retag-only first, then conditionally skip QEMU/Buildx setup and build steps when `mimosa-cache-hit` is true | `false` |

### Additional outputs

| Output | Description |
|--------|-------------|
| `mimosa-setup-binary-path` | Full path to the mimosa binary |
| `mimosa-cache-hit` | Whether the mimosa cache was hit (`true` or `false`) |

### Using retag-only to skip setup on cache hit

You can run the action with `mimosa-retag-only: true` first (after checkout and registry logins). When the cache is hit, that step outputs `mimosa-cache-hit: true` and retags the image without running Docker or Buildx. When the cache is missed, it outputs `mimosa-cache-hit: false` and exits successfully so your workflow can run QEMU/Buildx setup and a full build step only when needed. This avoids installing Docker Buildx and QEMU when every image is already cached. See the [example workflow](./example-github-action.yml) for a full pattern.

---

## Step-by-step way

If you need more control over the build process, you can use the individual actions.

### 1. Setup mimosa

```yaml
- id: setup-mimosa
  uses: hytromo/mimosa/gh/setup-action@v2-setup
  with:
    version: v0.1.2
```

### 2. Run your docker command with the `mimosa remember --` prefix

```yaml
  run: mimosa remember -- docker buildx build --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-testing:${{ github.sha }} .
```