#!/usr/bin/env python3
"""Scan concrete debt patterns called out by the sanitation plan."""

from __future__ import annotations

import re
from dataclasses import dataclass
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
EXCLUDE_DIRS = {
    ".git",
    ".venv",
    "node_modules",
    "dist",
    "__pycache__",
    ".pytest_cache",
    ".ruff_cache",
    "coverage",
}
DEBT_EXCLUDE_PARTS = {"migrations", "seeds", "generated", "tests", "docs"}
HOTSPOT_EXCLUDE_PARTS = {"generated", "tests", "docs"}
AI_FAKE_FALLBACK_CONTRACT_FILES = {
    Path("ai/src/agents/catalog.py"),
    Path("ai/src/api/chat_contract.py"),
    Path("scripts/audit/debt_scan.py"),
}
DEBT_SCAN_CONTRACT_FILES = {
    Path("scripts/audit/debt_scan.py"),
}
INCLUDE_SUFFIXES = {".go", ".ts", ".tsx", ".py", ".sql", ".md", ".sh"}


@dataclass(frozen=True)
class DebtPattern:
    key: str
    description: str
    regex: re.Pattern[str]
    globs: tuple[str, ...]


PATTERNS = (
    DebtPattern(
        "legacy_error_shape",
        'HTTP responses using gin.H{"error": ...} instead of {code,message}',
        re.compile(r'gin\.H\s*\{\s*"error"\s*:'),
        ("*.go",),
    ),
    DebtPattern(
        "inline_dto",
        "Inline request DTOs in handlers",
        re.compile(r"var\s+\w+\s+struct\s*\{"),
        ("*.go",),
    ),
    DebtPattern(
        "manual_limit_parse",
        "Manual limit parsing instead of shared parser",
        re.compile(r"DefaultQuery\(\s*\"limit\"|strconv\.Atoi\([^)]*limit"),
        ("*.go",),
    ),
    DebtPattern(
        "legacy_deprecated_markers",
        "Legacy/deprecated/shim/workaround markers in actionable code",
        re.compile(r"\b(legacy|deprecated|shim|workaround|compatibility|compatibilidad)\b", re.IGNORECASE),
        ("*.go", "*.ts", "*.tsx", "*.py", "*.sh", "*.md"),
    ),
    DebtPattern(
        "frontend_crud_compat",
        "Frontend CRUD compatibility knobs that should be temporary",
        re.compile(r"REST_ARCHIVE_VIA_POST|softArchiveHttp|hardDeleteHttp|archiveMode"),
        ("*.ts", "*.tsx"),
    ),
    DebtPattern(
        "ai_fake_fallback_risk",
        "AI fallback paths that may bypass strict evidence/LLM behavior",
        re.compile(r"_fallback|fallback_blocks|read_fallback|analysis_fallback", re.IGNORECASE),
        ("*.py",),
    ),
)


def iter_files() -> list[Path]:
    files: list[Path] = []
    for path in ROOT.rglob("*"):
        if not path.is_file() or path.suffix not in INCLUDE_SUFFIXES:
            continue
        if any(part in EXCLUDE_DIRS for part in path.relative_to(ROOT).parts):
            continue
        files.append(path)
    return sorted(files)


def matches_glob(path: Path, globs: tuple[str, ...]) -> bool:
    return any(path.match(glob) for glob in globs)


def count_pattern(files: list[Path], pattern: DebtPattern) -> tuple[int, list[str]]:
    count = 0
    examples: list[str] = []
    for path in files:
        if not matches_glob(path, pattern.globs):
            continue
        if pattern.key == "inline_dto":
            if path.name.endswith("_test.go") or path.name != "handler.go":
                continue
        if pattern.key in {"legacy_deprecated_markers", "ai_fake_fallback_risk"}:
            rel = path.relative_to(ROOT)
            rel_parts = set(rel.parts)
            if path.name.endswith(("_test.go", "_test.py", ".test.ts", ".test.tsx")) or rel_parts & DEBT_EXCLUDE_PARTS:
                continue
            if pattern.key == "legacy_deprecated_markers" and rel in DEBT_SCAN_CONTRACT_FILES:
                continue
            if pattern.key == "ai_fake_fallback_risk" and rel in AI_FAKE_FALLBACK_CONTRACT_FILES:
                continue
        try:
            text = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        for line_no, line in enumerate(text.splitlines(), 1):
            if pattern.regex.search(line):
                count += 1
                if len(examples) < 8:
                    rel = path.relative_to(ROOT)
                    examples.append(f"{rel}:{line_no}: {line.strip()[:140]}")
    return count, examples


def largest_files(files: list[Path], minimum_lines: int = 700) -> list[tuple[int, Path]]:
    result: list[tuple[int, Path]] = []
    for path in files:
        if path.suffix not in {".go", ".ts", ".tsx", ".py"}:
            continue
        if set(path.relative_to(ROOT).parts) & HOTSPOT_EXCLUDE_PARTS:
            continue
        try:
            line_count = path.read_text(encoding="utf-8").count("\n") + 1
        except UnicodeDecodeError:
            continue
        if line_count >= minimum_lines:
            result.append((line_count, path))
    return sorted(result, reverse=True)[:20]


def main() -> int:
    files = iter_files()
    print("# Technical debt scan")
    print()
    print("This scan is informational. It highlights concrete cleanup targets from the refactor plan.")
    print()
    print("| key | matches | description |")
    print("|---|---:|---|")
    details: list[tuple[DebtPattern, int, list[str]]] = []
    for pattern in PATTERNS:
        count, examples = count_pattern(files, pattern)
        details.append((pattern, count, examples))
        print(f"| {pattern.key} | {count} | {pattern.description} |")

    print()
    print("## Hotspots")
    print()
    print("| lines | file |")
    print("|---:|---|")
    for line_count, path in largest_files(files):
        print(f"| {line_count} | {path.relative_to(ROOT)} |")

    print()
    print("## Examples")
    for pattern, count, examples in details:
        if count == 0:
            continue
        print()
        print(f"### {pattern.key}")
        for example in examples:
            print(f"- {example}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
