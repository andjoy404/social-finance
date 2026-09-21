#!/bin/sh
# Entrypoint: copies the pre-built server binary into the working directory
# so it survives the ./backend:/app bind mount, then runs it.
set -e

exec "$@"
