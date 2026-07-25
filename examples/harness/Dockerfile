# Standalone image for the gadget widget harness.
#
# It takes no build context: the builder clones the repository itself, so the
# Dockerfile can be built straight from a URL or piped in on its own.
#
#   docker build -t gadget-harness https://github.com/Techthos/gadget.git#main:examples/harness
#   docker build -t gadget-harness - < examples/harness/Dockerfile
#   docker run --rm -p 8090:8090 gadget-harness
#
# Then open http://localhost:8090.
#
# Build a different branch, tag or commit:
#   docker build -t gadget-harness --build-arg GIT_REF=v0.2.0 - < examples/harness/Dockerfile
#
# The clone layer is cached; to pick up new commits on the same ref, pass
# --no-cache or --build-arg CACHEBUST="$(date +%s)".

ARG GO_VERSION=1.25

FROM golang:${GO_VERSION}-alpine AS build

ARG REPO_URL=https://github.com/Techthos/gadget.git
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
# Static binary so the runtime stage can be scratch. The harness serves only
# embedded assets, so it needs nothing else from the filesystem.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/harness ./examples/harness

FROM scratch

COPY --from=build /out/harness /harness

# No shell in the image, so there is no HEALTHCHECK; probe http://host:8090/
# from outside instead.
EXPOSE 8090
USER 65532:65532
ENTRYPOINT ["/harness"]
CMD ["-addr", ":8090"]
