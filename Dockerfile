FROM --platform=$BUILDPLATFORM quay.io/cybozu/golang:1.20-jammy as build-env

ARG TARGETARCH

COPY . /workdir
WORKDIR /workdir

RUN make build GOARCH=${TARGETARCH}

FROM --platform=$TARGETPLATFORM quay.io/cybozu/ubuntu:22.04

# https://docs.github.com/en/packages/managing-container-images-with-github-container-registry/connecting-a-repository-to-a-container-image#connecting-a-repository-to-a-container-image-on-the-command-line
LABEL org.opencontainers.image.source https://github.com/ysksuzuki/egress-gw-cni-plugin

RUN apt-get update \
    && apt-get install -y --no-install-recommends netbase kmod iptables iproute2 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build-env /workdir/work /usr/local/egress-gw

ENV PATH /usr/local/egress-gw:$PATH
