#!/bin/sh
set -e

if [ "$1" = "worker" ]; then
    echo "Starting worker..."
    exec /app/worker
elif [ "$1" = "webserver" ]; then
    echo "Starting webserver..."
    exec /app/webserver
else
    echo "Usage: $0 {worker|webserver}"
    exit 1
fi
