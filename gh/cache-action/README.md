# hytromo/mimosa/gh/cache-action

See [here](../../docs/gh-actions/README.md) for example usage.

This action uses a repository variable to store the Mimosa cache.

Note: this action is designed in a way that will never cause your workflow to
fail, even if the cache is not saved successfully. This follows Mimosa's
philosophy of being a nice-to-have, without disrupting the normal workflow.

## Inputs:

| Name            | Description                                                                  | Required | Default        |
| --------------- | ---------------------------------------------------------------------------- | -------- | -------------- |
| `github-token`  | GitHub token to use for authentication - needs to have write:variables scope | true     | -              |
| `variable-name` | The repository variable name to save the cache                               | false    | `MIMOSA_CACHE` |
| `max-length`    | The maximum length of the cache in bytes                                     | false    | `48000`        |

## Outputs:

| Name              | Description                                                            |
| ----------------- | ---------------------------------------------------------------------- |
| `new-cache-value` | The new calculated cache value saved back as a GitHub Actions variable |

## Repository Variable?

Yes! It can actually hold up to
[48KB of cache](https://docs.github.com/en/actions/reference/workflows-and-actions/variables#limits-for-configuration-variables) -
and that doesn't sound a lot, but if your cache looks like this:

```
k<1?mqOAOVX>aSpPL1:q 942291064371.dkr.ecr.us-east-1.amazonaws.com/mimosa-testing:recommended-example-75922fed99e86dbd086f0b984a9b3a5cc0b148e5
dx2?mqOAOPX>apSLP3?e 942291064371.dkr.ecr.us-east-1.amazonaws.com/mimosa-testing:recommended-40bf15ff32f0365f319064e758083431
... etc ...
```

with an average entry size of 142 bytes (like the 1st line in the above example)
we are able to save your 48000/142 ~= 338 most recent builds!

By using the default Github Actions cache you are limited to per-branch caching
only - this means that your `main`/`master` branch will always need to build
again before deploying - no image promotion.

## What happens if my cache gets too big?

The action automatically removes the oldest entries to make room for the new
ones so the cache will constantly stay below `max-length`.
