#!/bin/sh
set -e

# Substitute environment variables in nginx config
envsubst '${API_BACKEND_URL}' < /etc/nginx/nginx.conf.template > /etc/nginx/nginx.conf

exec nginx -g 'daemon off;'
