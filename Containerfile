ARG TEST_IMAGE="${TEST_IMAGE:-ghcr.io/ublue-os/base-main:41}"
ARG FEDORA_MAJOR_VERSION="${FEDORA_MAJOR_VERSION:-41}"

FROM registry.fedoraproject.org/fedora:${FEDORA_MAJOR_VERSION} AS builder

ENV UBLUE_ROOT=/app/output

WORKDIR /app 

ADD . /app

RUN dnf install -y just

RUN just container-rpm-build

FROM ${TEST_IMAGE}

ENV UBLUE_ROOT=/app/output


COPY --from=builder ${UBLUE_ROOT}/ublue-os/rpms /tmp/rpms
RUN dnf install -y /tmp/rpms/uupd.rpm

# FROM: https://github.com/containers/image_build/blob/main/podman/Containerfile, sets up podman to work in the container
RUN useradd -G wheel podman && \
    echo -e "podman:1:999\npodman:1001:64535" > /etc/subuid && \
    echo -e "podman:1:999\npodman:1001:64535" > /etc/subgid && \
    echo "podman:" | chpasswd

ADD ./containers.conf /etc/containers/containers.conf
ADD ./podman-containers.conf /home/podman/.config/containers/containers.conf

RUN mkdir -p /home/podman/.local/share/containers && \
    chown podman:podman -R /home/podman && \
    chmod 644 /etc/containers/containers.conf

# Copy & modify the defaults to provide reference if runtime changes needed.
# Changes here are required for running with fuse-overlay storage inside container.
RUN sed -e 's|^#mount_program|mount_program|g' \
           -e '/additionalimage.*/a "/var/lib/shared",' \
           -e 's|^mountopt[[:space:]]*=.*$|mountopt = "nodev,fsync=0"|g' \
           /usr/share/containers/storage.conf \
           > /etc/containers/storage.conf

# Setup internal Podman to pass subscriptions down from host to internal container
RUN printf '/run/secrets/etc-pki-entitlement:/run/secrets/etc-pki-entitlement\n/run/secrets/rhsm:/run/secrets/rhsm\n' > /etc/containers/mounts.conf

# Note VOLUME options must always happen after the chown call above
# RUN commands can not modify existing volumes
VOLUME /var/lib/containers
VOLUME /home/podman/.local/share/containers

RUN mkdir -p /var/lib/shared/overlay-images \
             /var/lib/shared/overlay-layers \
             /var/lib/shared/vfs-images \
             /var/lib/shared/vfs-layers && \
    touch /var/lib/shared/overlay-images/images.lock && \
    touch /var/lib/shared/overlay-layers/layers.lock && \
    touch /var/lib/shared/vfs-images/images.lock && \
    touch /var/lib/shared/vfs-layers/layers.lock

ENV _CONTAINERS_USERNS_CONFIGURED="" \
    BUILDAH_ISOLATION=chroot
# RUN useradd -m -G wheel user && echo "user:" | chpasswd

CMD [ "/sbin/init" ]
