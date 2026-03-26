#!/usr/bin/env bash
# Limpieza bajo el árbol de proyectos (por defecto padre de este repo: .../Projects/Pablo).
#
# Quita:
#   - __pycache__, .pytest_cache, .mypy_cache, .ruff_cache (no entra a .git, node_modules, .venv, venv)
#   - archivos vacíos (excepto .gitkeep)
#   - respaldos de editor *.swp, *~
#   - binarios ELF sueltos típicos de `go build` (main, backend, lambda, …) solo bajo */backend/* o */cmd/*
#   - ejecutables *.test (artefacto `go test -c`) ELF en esas mismas rutas
#   - directorios vacíos (iterativo), sin tocar .git
#
# Simulación: DRY_RUN=1 bash scripts/cleanup-pablo-tree.sh
# Otra raíz: bash scripts/cleanup-pablo-tree.sh /ruta/otro/monorepo

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
DEFAULT_BASE=$(cd "$SCRIPT_DIR/../.." && pwd)
BASE="${1:-$DEFAULT_BASE}"

if [[ ! -d "$BASE" ]]; then
  echo "No existe: $BASE" >&2
  exit 1
fi

echo "Base: $BASE"
if [[ -n "${DRY_RUN:-}" ]]; then
  echo "(modo DRY_RUN: solo se lista, no se borra)"
fi

rm_rf() {
  if [[ -n "${DRY_RUN:-}" ]]; then
    echo "DRY rm -rf $1"
  else
    rm -rf "$1"
  fi
}

rm_f() {
  if [[ -n "${DRY_RUN:-}" ]]; then
    echo "DRY rm -f $1"
  else
    rm -f "$1"
  fi
}

removed_files=0
removed_dirs=0

# Caches Python / herramientas (no node_modules ni venvs)
while IFS= read -r -d '' p; do
  rm_rf "$p"
  removed_dirs=$((removed_dirs + 1))
done < <(
  find "$BASE" \
    \( -type d -name .git -prune \) -o \
    \( -type d -name node_modules -prune \) -o \
    \( -type d -name .venv -prune \) -o \
    \( -type d -name venv -prune \) -o \
    \( -type d \( -name __pycache__ -o -name .pytest_cache -o -name .mypy_cache -o -name .ruff_cache \) -print0 \) \
    2>/dev/null
)

# Archivos vacíos (no .gitkeep)
while IFS= read -r -d '' f; do
  [[ $(basename "$f") == .gitkeep ]] && continue
  rm_f "$f"
  removed_files=$((removed_files + 1))
done < <(
  find "$BASE" \
    \( -type d -name .git -prune \) -o \
    \( -type d -name node_modules -prune \) -o \
    \( -type f -empty ! -name .gitkeep -print0 \) \
    2>/dev/null
)

# Backups de editor
while IFS= read -r -d '' f; do
  rm_f "$f"
  removed_files=$((removed_files + 1))
done < <(
  find "$BASE" \
    \( -type d -name .git -prune \) -o \
    \( -type d -name node_modules -prune \) -o \
    \( -type f \( -name '*.swp' -o -name '*~' \) -print0 \) \
    2>/dev/null
)

# Binarios Go sueltos (solo backend/cmd, solo ELF)
while IFS= read -r -d '' f; do
  case "$f" in
    */.git/* | */node_modules/*) continue ;;
  esac
  [[ "$f" == */backend/* || "$f" == */cmd/* ]] || continue
  bn=$(basename "$f")
  mt=$(file -b --mime-type "$f" 2>/dev/null || true)
  [[ "$mt" == application/x-executable || "$mt" == application/x-pie-executable ]] || continue
  case "$bn" in
    main | backend | server | lambda | local | cp-backend | work-backend | prof-backend | beauty-backend | restaurants-backend)
      rm_f "$f"
      echo "rm binary: $f"
      removed_files=$((removed_files + 1))
      ;;
    *.test)
      rm_f "$f"
      echo "rm test binary: $f"
      removed_files=$((removed_files + 1))
      ;;
  esac
done < <(
  find "$BASE" \
    \( -type d -name .git -prune \) -o \
    \( -type d -name node_modules -prune \) -o \
    \( -type f \( -path '*/backend/*' -o -path '*/cmd/*' \) -perm -111 -print0 \) \
    2>/dev/null
)

# Directorios vacíos (varias pasadas; en DRY_RUN una sola pasada listando)
if [[ -n "${DRY_RUN:-}" ]]; then
  while IFS= read -r -d '' d; do
    case "$d" in
      */.git | */.git/*) continue ;;
    esac
    echo "DRY rmdir $d"
    removed_dirs=$((removed_dirs + 1))
  done < <(find "$BASE" -depth -type d -empty ! -path '*/.git' ! -path '*/.git/*' -print0 2>/dev/null)
else
  for _ in $(seq 1 40); do
    n=0
    while IFS= read -r -d '' d; do
      case "$d" in
        */.git | */.git/*) continue ;;
      esac
      if rmdir "$d" 2>/dev/null; then
        echo "rmdir: $d"
        n=$((n + 1))
        removed_dirs=$((removed_dirs + 1))
      fi
    done < <(find "$BASE" -depth -type d -empty ! -path '*/.git' ! -path '*/.git/*' -print0 2>/dev/null)
    [[ "$n" -eq 0 ]] && break
  done
fi

echo "Listo. Archivos tocados (aprox): $removed_files, entradas de cache / dirs vacíos (aprox): $removed_dirs"
