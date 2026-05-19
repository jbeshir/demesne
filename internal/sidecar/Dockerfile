FROM gcr.io/distroless/static-debian12
COPY demesne-sidecar /usr/local/bin/demesne-sidecar
# Run as root inside the container so the bind-mounted /sidecar-results
# (owned by the host user) is writable. Under rootless podman, container
# root maps to the host user that started the daemon; under root podman /
# docker, root inside the namespace can still write the bind mount.
USER 0
ENTRYPOINT ["/usr/local/bin/demesne-sidecar"]
