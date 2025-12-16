# Attribution

This GitHub Action is based on [docker/build-push-action](https://github.com/docker/build-push-action).

## Original Work

- **Copyright:** 2013-2018 Docker, Inc.
- **License:** Apache License 2.0 (see [LICENSE](./LICENSE))
- **Repository:** https://github.com/docker/build-push-action

## Modifications

**Modified by:** Alexandros Solanos ([@hytromo](https://github.com/hytromo))  
**Year:** 2025

### Summary of Changes

- Integrated `mimosa remember` to wrap the Docker buildx command for build caching
- Modified `src/main.ts` to invoke builds through mimosa instead of directly calling buildx

## License

The original work is licensed under the Apache License 2.0. This derivative work retains the same license for the original portions. See the [LICENSE](./LICENSE) file for full license text.
