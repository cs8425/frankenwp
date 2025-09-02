#!/bin/bash
set -e

# wp core & test data
mkdir -p ./wp

# web server
mkdir -p ./caddy_data

# php-fpm
mkdir -p ./php-sockets

chmod 777 ./caddy_data ./wp ./php-sockets

# db 999:999
mkdir -p ./db ./sockets


docker compose -f $1 down
docker compose -f $1 up
docker compose -f $1 down

