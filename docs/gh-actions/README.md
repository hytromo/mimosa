# GitHub Actions

You can find full examples of both ways you can utilize Mimosa in Github Actions in the [example-github-action](./example-github-action.yml) workflow. See below for the important bits.

## Recommended way: build-push-action

The easiest way to use Mimosa is with the all-in-one `build-push-action`. It handles setup, building, and cache management automatically.

```yaml
- uses: hytromo/mimosa/gh/build-push-action@v6-build-push
  with:
    # Required: PAT with variables:write permission for repo-global cache
    # Create one here: https://github.com/settings/personal-access-tokens
    mimosa-cache-github-token: ${{ secrets.WRITE_VARIABLES_GH_PAT }}
    # All docker/build-push-action inputs are supported
    platforms: linux/amd64,linux/arm64
    push: true
    tags: my-org/my-image:${{ github.sha }}
```

This action wraps `docker/build-push-action` with Mimosa integration, so all the [standard inputs](https://github.com/docker/build-push-action#inputs) are supported. The tag of this action is following the official `docker/build-push-action` tag.

### Additional inputs

| Input | Description | Default |
|-------|-------------|---------|
| `mimosa-setup-enabled` | Enable/disable mimosa setup | `true` |
| `mimosa-setup-version` | Mimosa binary version | `latest` |
| `mimosa-setup-tools-file` | Path to .tool-versions file | `.tool-versions` |
| `mimosa-cache-enabled` | Enable/disable mimosa cache saving | `true` |
| `mimosa-cache-github-token` | GitHub token with `variables:write` scope | - |
| `mimosa-cache-variable-name` | Repository variable name for cache | `MIMOSA_CACHE` |
| `mimosa-cache-max-length` | Maximum length of the cache | `48000` |

### Additional outputs

| Output | Description |
|--------|-------------|
| `mimosa-setup-binary-path` | Full path to the mimosa binary |
| `mimosa-setup-cache-path` | Full path to the mimosa cache directory |
| `mimosa-cache-new-cache-value` | The new cache value saved to the repo variable |
| `mimosa-cache-hit` | Whether mimosa cache was hit |

---

## Step-by-step way

If you need more control over the build process, you can use the individual actions. Choose between two cache backends:

### Option A: Using mimosa-cache (recommended)

This uses a repo variable to store the cache, enabling a true repo-global cache across all branches.

#### 1. Setup mimosa

```yaml
- id: setup-mimosa
  uses: hytromo/mimosa/gh/setup-action@v2-setup
  with:
    version: v0.1.1
```

#### 2. Run your docker command with the `mimosa remember --` prefix

```yaml
- env:
    # Pass the repo variable here - even if it doesn't exist yet
    MIMOSA_CACHE: ${{ vars.MIMOSA_CACHE }}
  run: |
    mimosa remember -- docker buildx build --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-testing:${{ github.sha }} .
```

#### 3. Save the cache back

```yaml
- uses: hytromo/mimosa/gh/cache-action@v2-cache
  with:
    # Important: a PAT with variables:write permission is required
    # Create one here: https://github.com/settings/personal-access-tokens and add it to your repo secrets
    github-token: ${{ secrets.WRITE_VARIABLES_GH_PAT }}
```

### Option B: Using actions/cache (partial benefits)

If you don't want to use a repo variable, you can use `actions/cache` instead, with [its limitations](https://docs.github.com/en/actions/reference/dependency-caching-reference#restrictions-for-accessing-a-cache). This won't give you a true repo-global cache - your default branch will always need to rebuild. PRs that don't influence the build will still benefit though.

#### 1. Setup mimosa

```yaml
- id: setup-mimosa
  uses: hytromo/mimosa/gh/setup-action@v2-setup
  with:
    version: v0.1.1
```

#### 2. Fetch the cache

```yaml
- name: Fetch mimosa cache
  uses: actions/cache@v4
  with:
    path: ${{ steps.setup-mimosa.outputs.cache-path }}
    key: mimosa-cache-${{ github.ref }}-${{ github.sha }}
    restore-keys: |
      mimosa-cache-${{ github.ref }}-
      mimosa-cache-
```

The post-cache action of this step will also ensure that the cache will be saved back at the end.

#### 3. Run your docker command with the `mimosa remember --` prefix

```yaml
- run: |
    mimosa remember -- docker buildx build --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-testing:${{ github.sha }} .
```
