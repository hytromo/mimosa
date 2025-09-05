# syntax=docker/dockerfile:1.4

FROM alpine:latest AS final

WORKDIR /app

# Attach multiple contexts
# Each context uses its own .dockerignore
COPY . /app
COPY --from=frontend . /app
COPY --from=backend  . /app

CMD ["ls", "-R", "/app"]

