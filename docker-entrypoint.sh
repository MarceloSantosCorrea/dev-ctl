#!/bin/bash
set -e

# Cria symlink para que paths do host funcionem dentro do container (DooD)
if [ -n "$DEVCTL_HOST_DATA_DIR" ] && [ "$DEVCTL_HOST_DATA_DIR" != "$HOME/.devctl" ]; then
  host_parent=$(dirname "$DEVCTL_HOST_DATA_DIR")
  mkdir -p "$host_parent"
  if [ ! -e "$DEVCTL_HOST_DATA_DIR" ]; then
    ln -s "$HOME/.devctl" "$DEVCTL_HOST_DATA_DIR"
  fi
fi

# Init idempotente (cria dirs, copia templates, inicia Traefik, migra DB)
devctl init

# Executa o comando (default: serve)
exec devctl "$@"
