FROM ubuntu:22.04
WORKDIR /opt/env

ARG DEBIAN_FRONTEND=noninteractive
ENV TZ=Europe/Moscow
# RUN apt install gcc-9
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates locales g++ gcc libc6-dev make pkg-config wget git libopus-dev sox ffmpeg apt-utils tcpdump && \
    rm -rf /var/lib/apt/lists/*

ADD static /static
VOLUME /tmp

COPY main ./main
COPY conf.yaml ./conf.yaml
COPY static ./static

COPY pkg/anim/testdata/init.json ./init.json

ENTRYPOINT ["/bin/sh", "-c", "./main >> /tmp/sfu.txt 2>&1"]
#ENTRYPOINT ["/bin/sh", "-c", "ls -al /static/demo"]
