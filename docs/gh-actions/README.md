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

### Additional outputs

| Output | Description |
|--------|-------------|
| `mimosa-setup-binary-path` | Full path to the mimosa binary |

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