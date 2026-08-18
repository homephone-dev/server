#!/bin/sh
set -eu

: "${ARI_PASSWORD:?ARI_PASSWORD must be set}"

envsubst '${ARI_PASSWORD}' < /etc/asterisk/ari.conf.template > /etc/asterisk/ari.conf

exec "$@"
