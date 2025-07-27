<div align="center">
  <h1><a href="https://github.com/hytromo/mimosa">
      <img src="./logo.webp" alt="mimosa-logo" height="300" /><br />
      mimosa
    </a></h1>

  <a href="https://github.com/hytromo/mimosa/releases"><img alt="Github Releases"
      src="https://img.shields.io/badge/dynamic/json?url=https://api.github.com/repos/hytromo/mimosa/releases&query=$.0.name&style=for-the-badge&logo=go&label=mimosa"></a>
  <a href="https://github.com/hytromo/mimosa/blob/main/LICENSE"><img alt="GitHub" src="https://img.shields.io/github/license/hytromo/mimosa?color=%2344CC11&style=for-the-badge"></a>
  <a href="https://github.com/hytromo/mimosa/actions/workflows/main.yml"><img alt="GitHub Workflow Status" src="https://img.shields.io/github/actions/workflow/status/hytromo/mimosa/main.yml?style=for-the-badge"></a>
  <a href="https://github.com/hytromo/mimosa/stargazers"><img alt="GitHub Repo stars" src="https://img.shields.io/github/stars/hytromo/mimosa?style=for-the-badge
  "></a>
  <p><em>Zero-config docker image promotion</em></p>

</div>

# What does it do

* **No more wasteful docker builds** - you don't have to wait for CI to finish just because of a tiny change in the README.
* **Image promotion - for free** - you've tested your changes in a branch, so why should you wait for `main`/`master` to rebuild, and also risk producing a different image? Use Mimosa and skip straight to deployment.

# How does it do it

Just prepend your docker build commands like this: `mimosa remember -- docker buildx build ...` and Mimosa makes sure you don't run the same build twice. If you've run the same build before, it reaches out to your container registry and retags the previously built image instead.

<p align="center">
  <img width="800" src="./example.svg">
</p>


* It calculates a unique hash based on your docker command, build context, and your `Dockerfile`/`.dockerignore` files.  
* It saves the hash-tag map to the cache.  
* If it calculates the same hash in the future, it simply retags the already built image instead of rebuilding.  
* All without downloading or reuploading the image - that's what makes it fast.  
* If it hasn't seen the hash before, it builds the image as normal - no tricks. The hash is then saved to the cache for future use.

- [What does it do](#what-does-it-do)
- [How does it do it](#how-does-it-do-it)
- [Key Features](#key-features)
- [Installation](#installation)
  - [Inside GitHub Actions](#inside-github-actions)
  - [On your system](#on-your-system)
- [CLI usage](#cli-usage)
  - [Remember](#remember)
  - [Cache](#cache)
    - [Cache Management](#cache-management)
  - [Advanced usage](#advanced-usage)
    - [Log level](#log-level)
    - [Inject Cache via Env Variable](#inject-cache-via-env-variable)
- [FAQ](#faq)
  - [What about multi-platform builds?](#what-about-multi-platform-builds)
  - [What about custom build contexts?](#what-about-custom-build-contexts)
  - [What about custom Dockerfile locations?](#what-about-custom-dockerfile-locations)
  - [What about custom `.dockerignore` files?](#what-about-custom-dockerignore-files)
  - [Can I use normal docker build commands?](#can-i-use-normal-docker-build-commands)
  - [Isn't this just yet another way for my build to fail?](#isnt-this-just-yet-another-way-for-my-build-to-fail)
  - [What's up with the name?](#whats-up-with-the-name)
- [Contributing](#contributing)


# Key Features

- **Seamless docker Integration:**  
  Mimosa wraps standard `docker buildx build` (and `docker build`) commands. You use it by passing the same arguments you would to Docker.

- **Automatic Context and Dockerfile Detection:**  
  Mimosa automatically detects the build context, Dockerfile, and `.dockerignore` (including custom-named dockerignore files). It accounts for exactly what's needed to ensure that your build gets a unique cache key. It ignores all files specified in your `.dockerignore`, so a well-maintained `.dockerignore` makes all the difference.

- **Seamless Integration with GitHub Actions:**  
  `mimosa` works with `actions/cache` as well as its own `hytromo/mimosa/gh/cache-action` action, which offers repository-scoped caching [instead of branch-scoped](https://docs.github.com/en/actions/reference/dependency-caching-reference#restrictions-for-accessing-a-cache) - which means that all your branches can have access to the same cache at once!

# Installation

## Inside GitHub Actions

```yaml
- uses: hytromo/mimosa/gh/setup-action@setup-action-v1
  with:
    version: v0.0.9
```

See the [the GitHub Action docs](./docs/gh-actions/README.md) for details on how to use `mimosa` in your GitHub Actions.

## On your system

Pre-built binaries are available on the [Releases page](https://github.com/hytromo/mimosa/releases). Download the appropriate binary for your platform and add it to your `PATH`.

# CLI usage

## Remember

```sh
mimosa remember -- docker buildx build --build-arg MYARG=MYVALUE --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-example:v1 .
# ... typical docker build

# Now change something that should not influence the build result

mimosa remember -- docker buildx build --build-arg MYARG=MYVALUE --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-example:v2 .
# ... mimosa understands that nothing important has changed, so it just makes v2 point to v1 - they are the same image - no build happens!


# dry run - do not build, retag or write to cache, just show what would happen
mimosa remember -dry-run -- docker buildx build --build-arg MYARG=MYVALUE --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-example:v2 .
```

* The `remember` subcommand tells Mimosa to cache and reuse builds.
* The rest of the command is exactly what you'd pass to `docker build`.

## Cache

### Cache Management

```sh
mimosa cache --show # Show where the cache is being saved

# forget cache associated with a specific build - this influences the local cache only, it doesn't touch the remote registry
mimosa forget -- docker buildx build --build-arg MYARG=MYVALUE --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-example:v1 .

mimosa cache --forget 6M # Forget entries older than 6 months
mimosa cache --forget 24h # Forget entries older than 24 hours

mimosa cache --forget 1y --yes # No user prompt
mimosa cache --purge # Delete all entries
```

## Advanced usage

### Log level

You can use the `LOG_LEVEL` env variable to control the log level, use `LOG_LEVEL=debug` for debug logging.

### Inject Cache via Env Variable

The `MIMOSA_CACHE` env variable can be used and takes precedence over the default cache location.

```bash
export MIMOSA_CACHE=`cat <<EOF
<z85 hash1> <tag1>
<hash2> <tag2>
<hash3> <tag3>
...
EOF`

mimosa remember -- docker buildx build ... # etc
```

Note: the cache location is still used to save the build's cache, even if `MIMOSA_CACHE` is present.

# FAQ

## What about multi-platform builds?

Mimosa supports multi-platform builds. It looks into the existing docker image's index manifests and makes the new tag point to the same ones - no matter how many or which they are.

## What about custom build contexts?

Mimosa analyzes the docker build command and understands what your build context is, whether it's `.` or something else.

## What about custom Dockerfile locations?

If you specify `-f` / `--file`, it will use that file instead of the default `Dockerfile`.

## What about custom `.dockerignore` files?

`<Dockerfile name>.dockerignore` takes precedence over `.dockerignore` ([docs](https://docs.docker.com/build/concepts/context/#dockerignore-files)) - Mimosa knows these rules.

## Can I use normal docker build commands?

... or am I forced to use BUILDKIT?

You can absolutely use typical docker build commands.

```bash
mimosa remember -- docker build -t hytromo/mimosa-testing:simple-docker-build .
```

... but it is not best practice to `mimosa remember` a build that it is not pushed - because mimosa will always assume that the image has successfully been pushed to the target registry if the command it remembers finishes successfully.
So if your separate `docker push` command fails, there is no way to invalidate Mimosa's cache.
Also, if there is a cache hit, Mimosa will also retag the image in the remote registry - which might be confusing if `--push` is not present in the command. So it's always a good idea to `--push` your builds when you `mimosa remember` them.

## Isn't this just yet another way for my build to fail?

Mimosa takes all the possible precautions to just run the provided command if it fails to calculate the hash, or has any other kind of issue.
In fact, it is so permissive that it can run any command - freely:
```bash
# it will detect that this is not a docker command, and it will just run it, even respecting its exit code
mimosa remember -- echo I am run via mimosa
```

Mimosa will also always respect the exit code of the docker command and will exit with the same code, so you can rest assured that any custom scripts or behaviors will continue working as expected.

## What's up with the name?

*Mimosa pudica* is a plant that closes its leaves on touch to protect itself.

If the plant is repeatedly exposed to a non-harmful stimulus, such as a gentle touch or a repeated drop that doesn't cause actual damage, it will eventually stop responding. This phenomenon is called habituation, which is considered the simplest form of learning. The plant "learns" that the repeated disturbance is not a threat and thus doesn't need to expend energy on closing its leaves.

# Contributing

If you are interested in contributing:

1. Install [mise-en-place](https://mise.jdx.dev/getting-started.html) - great for tool management
2. Clone the repo: `git clone https://github.com/hytromo/mimosa.git`
3. Initialize the whole project without polluting your global environment: `mise init`
4. Go tests are also testing real integration with dockerhub, you will need to be authenticated into dockerhub and the repository mimosa-testing will be created
5. Start hacking! The pre-commit hooks will help ensuring that the github actions are bundled or that the go code does not have code smells. Have a look at `.pre-commit-config.yaml` for details

