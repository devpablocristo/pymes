#!/bin/sh
# Sincroniza node_modules con package.json montado desde el host (evita rebuild por cada dependencia nueva).
set -e
cd /workspace/frontend
npm install
exec npm run dev -- --host 0.0.0.0
