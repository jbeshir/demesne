FROM ubuntu:24.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        ffmpeg \
        libavcodec-extra60 \
        imagemagick \
        libmagickcore-6.q16-7-extra \
        ghostscript \
        libvips-tools \
        webp \
        librsvg2-bin \
        potrace \
        gifsicle \
        libimage-exiftool-perl \
        sox \
        libsox-fmt-all \
        lame \
        flac \
        opus-tools \
        mediainfo \
        jpegoptim \
        pngquant \
        optipng \
        fontconfig \
        fonts-dejavu-core \
    && rm -rf /var/lib/apt/lists/*
