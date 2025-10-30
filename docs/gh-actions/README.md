# GitHub Actions

You can find full examples of both ways you can utilize Mimosa in Github Actions in the [example-github-action](./example-github-action.yml) workflow. See below for the important bits.

## Recommended way

The recommended way is to use the repo variable cache. This enables you to have a true repo-global cache - between all branches!

1. Setup mimosa

```yaml
- id: setup-mimosa
  uses: hytromo/mimosa/gh/setup-action@v1-setup
  with:
    version: v0.1.0
```

2. Run your docker command with the `mimosa remember --` prefix

```yaml
- env:
    # remember to pass the repo variable here :) - even if it doesn't exist yet
    MIMOSA_CACHE: ${{ vars.MIMOSA_CACHE }}
  run: |
    mimosa remember -- docker buildx build --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-testing:${{ github.sha }} .
```

3. Save the cache back

```yaml
- uses: hytromo/mimosa/gh/cache-action@v1-cache
  with:
    # important: a PAT with variables:write permission is required
    # create one here: https://github.com/settings/personal-access-tokens and add it to your repo secrets
    github-token: ${{ secrets.WRITE_VARIABLES_GH_PAT }}
```

## Not-recommended way (partial benefits)

If you - for any reason - don't want to use a repo variable, you can use `actions/cache` instead, with [its limitations](https://docs.github.com/en/actions/reference/dependency-caching-reference#restrictions-for-accessing-a-cache). This won't allow you to have a true repo-global cache. Your default branch will always need to rebuild. Your PRs that do changes that don't influence the build will still benefit from this, though.

1. Setup mimosa

```yaml
- id: setup-mimosa
  uses: hytromo/mimosa/gh/setup-action@v1-setup
  with:
    version: v0.1.0
```

2. Fetch the cache
  
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

3. Run your docker command with the `mimosa remember --` prefix

```yaml
- run: |
    mimosa remember -- docker buildx build --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-testing:${{ github.sha }} .
```