ARG WORDPRESS_VERSION=latest
ARG PHP_VERSION=8.4
ARG USER=www-data



FROM dunglas/frankenphp:builder-php${PHP_VERSION} AS builder

# Copy xcaddy in the builder image
COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

COPY ./sidekick/middleware/cache ./cache

# CGO must be enabled to build FrankenPHP
RUN CGO_ENABLED=1 XCADDY_SETCAP=1 XCADDY_GO_BUILD_FLAGS='-ldflags="-w -s" -trimpath' \
    CGO_CFLAGS=$(php-config --includes) \
    CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
    xcaddy build \
    --output /usr/local/bin/frankenphp \
    --with github.com/dunglas/frankenphp=./ \
    --with github.com/dunglas/frankenphp/caddy=./caddy/ \
    --with github.com/dunglas/caddy-cbrotli \
    # Add extra Caddy modules here
    --with github.com/stephenmiracle/frankenwp/sidekick/middleware/cache=./cache


FROM wordpress:$WORDPRESS_VERSION AS wp
FROM dunglas/frankenphp:php${PHP_VERSION} AS base

LABEL org.opencontainers.image.title=FrankenWP
LABEL org.opencontainers.image.description="Optimized WordPress containers to run everywhere. Built with FrankenPHP & Caddy."
LABEL org.opencontainers.image.url=https://wpeverywhere.com
LABEL org.opencontainers.image.source=https://github.com/StephenMiracle/frankenwp
LABEL org.opencontainers.image.licenses=MIT
LABEL org.opencontainers.image.vendor="Stephen Miracle"

ARG USER

ENV FORCE_HTTPS=0
ENV PHP_INI_SCAN_DIR=$PHP_INI_DIR/conf.d


RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    ghostscript \
    curl \
    libonig-dev \
    libxml2-dev \
    libcurl4-openssl-dev \
    libssl-dev \
    libzip-dev \
    unzip \
    git \
    less \
    libmimalloc3 \
    libjpeg-dev \
    libwebp-dev \
    libzip-dev \
    libmemcached-dev \
    zlib1g-dev


# install the PHP extensions we need (https://make.wordpress.org/hosting/handbook/handbook/server-environment/#php-extensions)
RUN install-php-extensions \
    bcmath \
    exif \
    gd \
    intl \
    mysqli \
    zip \
    imagick \
    opcache \
    # iconv \
    # openssl \
    # sodium \
    # sqlite3 \
    # zlib \
    # pdo \
    # pdo_sqlite \
    pdo_pgsql \
    pdo_mysql \
    # pdo_sqlsrv \ # wait microsoft update for debian trixie/13
    pdo_odbc \
    pgsql \
    bz2 \
    apcu \
    brotli \
    zstd \
    igbinary \
    msgpack \
    redis \
    memcached

COPY --from=wp /usr/src/wordpress /usr/src/wordpress
COPY --from=wp /usr/local/etc/php/conf.d /usr/local/etc/php/conf.d/
COPY --from=wp /usr/local/bin/docker-entrypoint.sh /usr/local/bin/

RUN sed -i \
    -e 's/\[ "$1" = '\''php-fpm'\'' \]/\[\[ "$1" == frankenphp* \]\]/g' \
    -e 's/php-fpm/frankenphp/g' \
    /usr/local/bin/docker-entrypoint.sh

# Add $_SERVER['ssl'] = true; when env USE_SSL = true is set to the wp-config.php file here: /usr/local/bin/wp-config-docker.php
RUN sed -i 's/<?php/<?php if (!!getenv("FORCE_HTTPS")) { \$_SERVER["HTTPS"] = "on"; } define( "FS_METHOD", "direct" ); set_time_limit(300); /g' /usr/src/wordpress/wp-config-docker.php

#RUN chown -R ${USER}:${USER} /usr/src/wordpress && \
#    chown -R ${USER}:${USER} /usr/local/bin/docker-entrypoint.sh


# Adding WordPress CLI
RUN curl -O https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar && \
    chmod +x wp-cli.phar && \
    mv wp-cli.phar /usr/local/bin/wp-cli

# fix wp cli
RUN echo '#!/bin/bash\nfrankenphp php-cli /usr/local/bin/wp-cli "$@"' > /usr/bin/wp && \
    chmod +x /usr/bin/wp

RUN cp $PHP_INI_DIR/php.ini-production $PHP_INI_DIR/php.ini
COPY php.ini $PHP_INI_DIR/conf.d/wp.ini

# Replace the official binary by the one contained your custom modules
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp

# docker-entrypoint.sh will copy /usr/src/wordpress to /var/www/html when first startup
COPY wp-content/mu-plugins /usr/src/wordpress/wp-content/mu-plugins
RUN mkdir /usr/src/wordpress/wp-content/cache

WORKDIR /var/www/html

VOLUME /var/www/html

COPY Caddyfile /etc/caddy/Caddyfile

# Caddy requires an additional capability to bind to port 80 and 443
RUN useradd -D ${USER} || true && \
    setcap CAP_NET_BIND_SERVICE=+eip /usr/local/bin/frankenphp

# Caddy requires write access to /data/caddy and /config/caddy
RUN chown -R ${USER}:${USER} /data/caddy && \
    chown -R ${USER}:${USER} /config/caddy && \
    chown -R ${USER}:${USER} /var/www/html

USER $USER

# use mimalloc by LD_PRELOAD
ENV LD_PRELOAD=/usr/lib/x86_64-linux-gnu/libmimalloc.so.3

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["frankenphp", "run", "--config", "/etc/caddy/Caddyfile"]
