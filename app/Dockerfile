# Minimal binary-donor image for envx.
#
# This image is a `scratch` base containing ONLY the statically linked envx
# binary — no shell, no libc, no CA certificates. It exists so other Dockerfiles
# can copy the binary out of it in a build stage, instead of downloading and
# verifying a release archive themselves:
#
#   COPY --from=ghcr.io/go-envx/envx:1.0.1 /envx /usr/local/bin/envx
#
# GoReleaser builds the binaries for every target platform (see the `dockers_v2`
# block in .goreleaser.yaml); this Dockerfile only copies the one matching the
# platform being built. Do NOT add a build stage here — that would rebuild the
# binary GoReleaser already produced.
FROM scratch

# Predefined buildx build arg (e.g. "linux/amd64"). It selects which prebuilt
# binary to copy from the build context, which GoReleaser lays out per platform.
ARG TARGETPLATFORM

COPY $TARGETPLATFORM/envx /envx

ENTRYPOINT ["/envx"]
