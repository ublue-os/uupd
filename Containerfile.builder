ARG FEDORA_MAJOR_VERSION="${FEDORA_MAJOR_VERSION:-41}"

FROM registry.fedoraproject.org/fedora:${FEDORA_MAJOR_VERSION} AS builder

ENV UBLUE_ROOT=/app/output

WORKDIR /app 

ADD . /app

RUN dnf install -y just git

RUN just container-rpm-build

FROM scratch

ENV UBLUE_ROOT=/app/output
COPY --from=builder ${UBLUE_ROOT}/ublue-os/rpms /tmp/rpms
