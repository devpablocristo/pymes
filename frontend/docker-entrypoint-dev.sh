#!/bin/sh
# En Docker-first las dependencias se resuelven en el build de la imagen.
# Evitamos `npm install` en cada arranque porque rompe con paquetes locales no publicados.
set -e
cd /workspace/pymes/frontend
exec npm run dev -- --host 0.0.0.0
