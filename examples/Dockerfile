# Standalone image for the gomukit widget harness and preview MCP server.
#
# It takes no build context: the builder clones the repository itself, so the
# Dockerfile can be built straight from a URL or piped in on its own.
#
#   docker build -t gomukit https://github.com/Techthos/gomukit.git#main:examples
#   docker build -t gomukit - < examples/Dockerfile
#
# The image carries two binaries and defaults to the harness:
#
#   docker run --rm -p 8090:8090 gomukit                      # harness, http://localhost:8090
#   docker run --rm -p 8081:8081 --entrypoint /preview gomukit \
#     -addr :8081 -sandbox -cors -session-timeout 30m        # MCP at http://localhost:8081/mcp
#
# -sandbox gives every MCP session its own copy of the scenario data, so a
# publicly reachable preview stays read-only in the sense that matters: a
# visitor can press every button, and nothing they do is visible to anyone
# else. -cors lets browser-based MCP clients connect cross-origin.
#
# docker-compose.yml in this directory runs both from the one image.
#
# Build a different branch, tag or commit:
#   docker build -t gomukit --build-arg GIT_REF=v0.2.0 - < examples/Dockerfile
#
# The clone layer is cached; to pick up new commits on the same ref, pass
# --no-cache or --build-arg CACHEBUST="$(date +%s)".

ARG GO_VERSION=1.25

FROM golang:${GO_VERSION}-alpine AS build

ARG REPO_URL=https://github.com/Techthos/gomukit.git
ARG GIT_REF=main
ARG CACHEBUST=0

RUN apk add --no-cache git

WORKDIR /src
# --branch takes branches and tags; a raw commit SHA needs a full clone first.
RUN echo "cachebust ${CACHEBUST}" >/dev/null && \
    { git clone --depth 1 --branch "${GIT_REF}" "${REPO_URL}" . || \
      { rm -rf /src/* /src/.[!.]* 2>/dev/null; \
        git clone "${REPO_URL}" . && git checkout --detach "${GIT_REF}"; }; }

RUN go mod download
# Static binaries so the runtime stage can be scratch. Both serve only
# embedded assets and in-memory fixtures, so they need nothing else from the
# filesystem.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/harness ./examples/harness && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/preview ./examples/preview

FROM scratch

COPY --from=build /out/harness /harness
COPY --from=build /out/preview /preview

# No shell in the image, so there is no HEALTHCHECK. Probe the harness at
# http://host:8090/ and the preview at http://host:8081/healthz from outside.
EXPOSE 8090 8081
USER 65532:65532
ENTRYPOINT ["/harness"]
CMD ["-addr", ":8090"]
