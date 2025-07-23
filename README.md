- [mimosa](#mimosa)
  - [Key Features](#key-features)
  - [Installation](#installation)
  - [Github actions](#github-actions)
  - [How It Works](#how-it-works)
  - [Example Usage](#example-usage)
    - [Remember](#remember)
    - [Forget](#forget)
  - [License](#license)

# mimosa


**Mimosa** is a smart caching wrapper for Docker image builds. It accelerates repeated builds by detecting when the build context and command are unchanged, and reuses previously built images by retagging them, avoiding unnecessary rebuilds. In the demo below, when there is a cache hit, the `docs-v2` tag is created by referencing the `docs` tag manifests, and the tag becames available in Dockerhub without pulling/pushing any image.

<p align="center">
  <img width="800" src="./example.svg">
</p>


## Key Features

- **Intelligent Build Caching:**  
  Mimosa computes a unique cache key based on the Docker build command (excluding the tag) and the exact set of files included in the build context (respecting `.dockerignore` and custom Dockerfile locations). If nothing relevant has changed, it can instantly retag a previously built image instead of running a full build.

- **Seamless Docker Integration:**  
  Mimosa wraps the standard `docker build` (and `docker buildx`) commands. You use it by passing the same arguments you would to Docker.

- **Automatic Context and Dockerfile Detection:**  
  Mimosa automatically detects the build context, Dockerfile, and `.dockerignore` (including custom-named dockerignore files).

- **Persistent Local Cache:**  
  Caching metadata is stored in a user-specific cache directory, tracking the last 10 tags for each unique build context and command.

- **Transparent Fallback:**  
  If the cache is not valid (e.g., files or command changed), Mimosa simply runs the Docker build as normal and updates the cache.

## Installation

Pre-built binaries are available on the [Releases page](https://github.com/hytromo/mimosa/releases). Download the appropriate binary for your platform and add it to your PATH.


## Github actions

See the [example github action](./.github/workflows/example-github-action.yml) for details on how to use `mimosa` in your Github Actions.

## How It Works

1. **Parse the Build Command:**  
   Mimosa analyzes your `docker build` command, extracting the context, Dockerfile, tag, and all files that will be sent to Docker.

2. **Compute a Cache Key:**  
   It hashes the build command (with the tag replaced by a placeholder) and all included files.

3. **Cache Lookup:**  
   - If a matching cache entry exists, Mimosa retags the previously built image to the requested tag and exits quickly.
   - If not, it runs the Docker build, then saves the result in the cache for future use.

4. **Result Reporting:**  
   Mimosa logs whether a cache hit or miss occurred.

## Example Usage

### Remember

```sh
mimosa remember -- docker buildx build --build-arg DATE=1753095178 --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-example:local-test-v2 .
```

- The `remember` subcommand tells Mimosa to cache and reuse builds.
- The rest of the command is exactly what you would pass to `docker build`.

### Forget

```sh
mimosa cache --forget 6M # forget entries older than 6 months
mimosa cache --forget 24h # forget entries older than 24 hours

mimosa cache --forget 1y --yes # no user input
```


## License

This project is licensed under the [GNU GPL v3](LICENSE). See the LICENSE file for details.
