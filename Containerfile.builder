ARG FEDORA_MAJOR_VERSION="${FEDORA_MAJOR_VERSION:-45}"

FROM registry.fedoraproject.org/fedora:${FEDORA_MAJOR_VERSION} AS builder

ENV UBLUE_ROOT=/app/output

WORKDIR /app 

ADD . /app

RUN --mount=type=cache,dst=/var/cache/libdnf5 \
  dnf config-manager setopt keepcache=1 && \
  dnf install -y just git

RUN --mount=type=cache,dst=/var/cache/libdnf5 \
  just container-rpm-build

FROM scratch

ENV UBLUE_ROOT=/app/output
COPY --from=builder ${UBLUE_ROOT}/ublue-os/rpms /rpms
