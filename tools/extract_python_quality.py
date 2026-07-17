#!/usr/bin/env python3
"""Extract evidence-backed Python quality signals for visualization experiments.

This is a deliberately small adapter, not a universal quality judge. It combines
Python AST measurements, a coverage.py XML report, and Git history. The output
keeps raw values and the transparent review-priority formula so a renderer can
show *why* something is prominent.
"""

from __future__ import annotations

import argparse
import ast
from collections import Counter, defaultdict
from datetime import datetime, timezone
import json
from pathlib import Path
import subprocess
from typing import Any, Iterable
import xml.etree.ElementTree as ET

from extract_python_facts import classify_call, dotted_name


DECISION_NODES = (
    ast.If,
    ast.For,
    ast.AsyncFor,
    ast.While,
    ast.With,
    ast.AsyncWith,
    ast.ExceptHandler,
    ast.IfExp,
    ast.Assert,
    ast.Match,
)


def percentile_ranks(values: dict[str, float]) -> dict[str, float]:
    """Return stable 0..1 mid-ranks; ties receive the same rank."""
    if not values:
        return {}
    ordered = sorted(values.values())
    if len(ordered) == 1:
        return {key: 0.0 for key in values}
    positions: dict[float, list[int]] = defaultdict(list)
    for index, value in enumerate(ordered):
        positions[value].append(index)
    rank = {
        value: (sum(indexes) / len(indexes)) / (len(ordered) - 1)
        for value, indexes in positions.items()
    }
    return {key: round(rank[value], 4) for key, value in values.items()}


class FunctionSignals(ast.NodeVisitor):
    """Collect metrics for one function without entering nested definitions."""

    def __init__(self) -> None:
        self.complexity = 1
        self.depth = 0
        self.max_nesting = 0
        self.effects: Counter[str] = Counter()

    def visit_FunctionDef(self, node: ast.FunctionDef) -> None:  # noqa: N802
        return

    def visit_AsyncFunctionDef(self, node: ast.AsyncFunctionDef) -> None:  # noqa: N802
        return

    def visit_ClassDef(self, node: ast.ClassDef) -> None:  # noqa: N802
        return

    def _nested(self, node: ast.AST) -> None:
        self.complexity += 1
        self.depth += 1
        self.max_nesting = max(self.max_nesting, self.depth)
        self.generic_visit(node)
        self.depth -= 1

    def visit_If(self, node: ast.If) -> None:  # noqa: N802
        self._nested(node)

    def visit_For(self, node: ast.For) -> None:  # noqa: N802
        self._nested(node)

    def visit_AsyncFor(self, node: ast.AsyncFor) -> None:  # noqa: N802
        self._nested(node)

    def visit_While(self, node: ast.While) -> None:  # noqa: N802
        self._nested(node)

    def visit_With(self, node: ast.With) -> None:  # noqa: N802
        self._nested(node)

    def visit_AsyncWith(self, node: ast.AsyncWith) -> None:  # noqa: N802
        self._nested(node)

    def visit_ExceptHandler(self, node: ast.ExceptHandler) -> None:  # noqa: N802
        self._nested(node)

    def visit_IfExp(self, node: ast.IfExp) -> None:  # noqa: N802
        self._nested(node)

    def visit_Assert(self, node: ast.Assert) -> None:  # noqa: N802
        self.complexity += 1
        self.generic_visit(node)

    def visit_Match(self, node: ast.Match) -> None:  # noqa: N802
        self.complexity += max(1, len(node.cases))
        self.depth += 1
        self.max_nesting = max(self.max_nesting, self.depth)
        self.generic_visit(node)
        self.depth -= 1

    def visit_BoolOp(self, node: ast.BoolOp) -> None:  # noqa: N802
        self.complexity += max(0, len(node.values) - 1)
        self.generic_visit(node)

    def visit_comprehension(self, node: ast.comprehension) -> None:
        self.complexity += 1 + len(node.ifs)
        self.generic_visit(node)

    def visit_Call(self, node: ast.Call) -> None:  # noqa: N802
        name = dotted_name(node.func)
        if name:
            for effect in classify_call(node, name):
                self.effects[effect] += 1
        self.generic_visit(node)


def iter_functions(tree: ast.AST) -> Iterable[tuple[str, ast.FunctionDef | ast.AsyncFunctionDef]]:
    def walk(nodes: Iterable[ast.stmt], parent: str = "") -> Iterable[tuple[str, Any]]:
        for node in nodes:
            if isinstance(node, ast.ClassDef):
                prefix = f"{parent}.{node.name}" if parent else node.name
                yield from walk(node.body, prefix)
            elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
                name = f"{parent}.{node.name}" if parent else node.name
                yield name, node
    return walk(getattr(tree, "body", []))


def module_name(relative: Path) -> str:
    parts = list(relative.with_suffix("").parts)
    if parts and parts[-1] == "__init__":
        parts.pop()
    return ".".join(parts)


def imports_for(tree: ast.AST, current_module: str) -> set[str]:
    imports: set[str] = set()
    current_package = current_module.split(".")[:-1]
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            imports.update(alias.name for alias in node.names)
        elif isinstance(node, ast.ImportFrom):
            if node.level:
                keep = max(0, len(current_package) - node.level + 1)
                base = current_package[:keep]
                if node.module:
                    base.extend(node.module.split("."))
                prefix = ".".join(base)
            else:
                prefix = node.module or ""
            if prefix:
                imports.add(prefix)
            for alias in node.names:
                imports.add(".".join(part for part in (prefix, alias.name) if part))
    return imports


def parse_coverage(path: Path | None) -> tuple[dict[str, dict[str, int]], dict[str, Any] | None]:
    if path is None or not path.exists():
        return {}, None
    root = ET.parse(path).getroot()
    result: dict[str, dict[str, int]] = {}
    for cls in root.findall(".//class"):
        filename = cls.attrib.get("filename", "")
        lines = cls.find("lines")
        if lines is None:
            continue
        total = covered = branch_total = branch_covered = 0
        for line in lines:
            total += 1
            covered += int(line.attrib.get("hits", "0")) > 0
            if line.attrib.get("branch") == "true":
                coverage = line.attrib.get("condition-coverage", "")
                if "(" in coverage and "/" in coverage:
                    fraction = coverage.split("(", 1)[1].split(")", 1)[0]
                    hit, possible = fraction.split("/", 1)
                    branch_covered += int(hit)
                    branch_total += int(possible)
        result[Path(filename).as_posix()] = {
            "lines_total": total,
            "lines_covered": covered,
            "branches_total": branch_total,
            "branches_covered": branch_covered,
        }
    metadata = {
        "path": str(path.resolve()),
        "modified_at": datetime.fromtimestamp(path.stat().st_mtime, timezone.utc).isoformat(),
        "line_rate": float(root.attrib.get("line-rate", 0)),
        "branch_rate": float(root.attrib.get("branch-rate", 0)),
    }
    return result, metadata


def parse_git_history(root: Path, since: str) -> dict[str, dict[str, int]]:
    command = [
        "git",
        "log",
        f"--since={since}",
        "--numstat",
        "--format=format:__VCM_COMMIT__",
        "--",
        ".",
    ]
    try:
        completed = subprocess.run(
            command,
            cwd=root,
            check=True,
            capture_output=True,
            text=True,
        )
    except (subprocess.CalledProcessError, FileNotFoundError):
        return {}

    result: dict[str, dict[str, int]] = defaultdict(lambda: {"commits": 0, "lines_changed": 0})
    seen_in_commit: set[str] = set()
    for raw in completed.stdout.splitlines():
        if raw == "__VCM_COMMIT__":
            seen_in_commit = set()
            continue
        parts = raw.split("\t")
        if len(parts) != 3 or not parts[0].isdigit() or not parts[1].isdigit():
            continue
        added, deleted, filename = parts
        filename = Path(filename).as_posix()
        result[filename]["lines_changed"] += int(added) + int(deleted)
        if filename not in seen_in_commit:
            result[filename]["commits"] += 1
            seen_in_commit.add(filename)
    return dict(result)


def group_for(path: Path) -> str:
    parts = list(path.parts)
    try:
        app_index = parts.index("app")
    except ValueError:
        return parts[0] if parts else "root"
    tail = parts[app_index + 1 : -1]
    if not tail:
        return "app-root"
    if tail[0] == "services" and len(tail) > 1:
        return "/".join(tail[:2])
    return tail[0]


def match_coverage(relative: str, coverage: dict[str, dict[str, int]]) -> dict[str, int]:
    if relative in coverage:
        return coverage[relative]
    matches = [value for key, value in coverage.items() if relative.endswith(key) or key.endswith(relative)]
    return matches[0] if len(matches) == 1 else {}


def collect_file(path: Path, root: Path, coverage: dict[str, dict[str, int]], history: dict[str, dict[str, int]]) -> dict[str, Any]:
    relative = path.relative_to(root).as_posix()
    source = path.read_text(encoding="utf-8")
    tree = ast.parse(source, filename=str(path))
    functions: list[dict[str, Any]] = []
    file_effects: Counter[str] = Counter()
    for name, node in iter_functions(tree):
        visitor = FunctionSignals()
        for statement in node.body:
            visitor.visit(statement)
        file_effects.update(visitor.effects)
        functions.append(
            {
                "name": name,
                "line": node.lineno,
                "end_line": getattr(node, "end_lineno", node.lineno),
                "lines": getattr(node, "end_lineno", node.lineno) - node.lineno + 1,
                "complexity": visitor.complexity,
                "max_nesting": visitor.max_nesting,
                "effects": dict(sorted(visitor.effects.items())),
            }
        )
    cov = match_coverage(relative, coverage)
    executable = cov.get("lines_total", 0)
    covered = cov.get("lines_covered", 0)
    branches = cov.get("branches_total", 0)
    branches_covered = cov.get("branches_covered", 0)
    code_lines = sum(1 for line in source.splitlines() if line.strip() and not line.lstrip().startswith("#"))
    current_module = module_name(Path(relative))
    imports = imports_for(tree, current_module)
    return {
        "path": relative,
        "group": group_for(Path(relative)),
        "module": current_module,
        "lines": len(source.splitlines()),
        "code_lines": code_lines,
        "functions": functions,
        "function_count": len(functions),
        "max_function_lines": max((item["lines"] for item in functions), default=0),
        "complexity_total": sum(item["complexity"] for item in functions),
        "complexity_max": max((item["complexity"] for item in functions), default=0),
        "nesting_max": max((item["max_nesting"] for item in functions), default=0),
        "effects": dict(sorted(file_effects.items())),
        "effect_sites": sum(file_effects.values()),
        "mutating_effect_sites": sum(
            count
            for effect, count in file_effects.items()
            if effect not in {"filesystem.read", "database.read", "telemetry.log"}
        ),
        "coverage": {
            "available": bool(cov),
            "lines_total": executable,
            "lines_covered": covered,
            "line_rate": round(covered / executable, 4) if executable else None,
            "uncovered_lines": executable - covered,
            "branches_total": branches,
            "branches_covered": branches_covered,
            "branch_rate": round(branches_covered / branches, 4) if branches else None,
        },
        "history": history.get(relative, {"commits": 0, "lines_changed": 0}),
        "imports": sorted(imports),
        "fan_out": 0,
        "fan_in": 0,
    }


def add_coupling(files: list[dict[str, Any]]) -> None:
    modules = {item["module"]: item for item in files}
    aliases: dict[str, str] = {}
    for module in modules:
        aliases[module] = module
        if "." in module:
            # Repositories commonly keep the import root below a project
            # directory (for example photochecker/app -> import app.*).
            aliases[module.split(".", 1)[1]] = module
    incoming: Counter[str] = Counter()
    for item in files:
        targets: set[str] = set()
        for imported in item.pop("imports"):
            candidates = [
                canonical
                for alias, canonical in aliases.items()
                if imported == alias or imported.startswith(f"{alias}.")
            ]
            if candidates:
                targets.add(max(candidates, key=len))
        targets.discard(item["module"])
        item["fan_out"] = len(targets)
        item["dependencies"] = sorted(modules[target]["path"] for target in targets)
        for target in targets:
            incoming[target] += 1
    for item in files:
        item["fan_in"] = incoming[item["module"]]


def add_attention(files: list[dict[str, Any]]) -> None:
    metrics = {
        "complexity": {item["path"]: item["complexity_max"] for item in files},
        "uncovered": {item["path"]: item["coverage"]["uncovered_lines"] for item in files},
        "churn": {item["path"]: item["history"]["lines_changed"] for item in files},
        "effects": {item["path"]: item["mutating_effect_sites"] for item in files},
        "coupling": {item["path"]: item["fan_in"] + item["fan_out"] for item in files},
    }
    ranks = {name: percentile_ranks(values) for name, values in metrics.items()}
    weights = {"complexity": 0.26, "uncovered": 0.24, "churn": 0.22, "effects": 0.16, "coupling": 0.12}
    for item in files:
        factors = {name: ranks[name][item["path"]] for name in weights}
        item["attention"] = {
            "value": round(sum(factors[name] * weight for name, weight in weights.items()), 4),
            "factors": factors,
        }


def aggregate_groups(files: list[dict[str, Any]]) -> list[dict[str, Any]]:
    grouped: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for item in files:
        grouped[item["group"]].append(item)
    result = []
    for name, members in sorted(grouped.items()):
        covered = sum(item["coverage"]["lines_covered"] for item in members)
        executable = sum(item["coverage"]["lines_total"] for item in members)
        branches_covered = sum(item["coverage"]["branches_covered"] for item in members)
        branches = sum(item["coverage"]["branches_total"] for item in members)
        result.append(
            {
                "id": name,
                "files": len(members),
                "lines": sum(item["lines"] for item in members),
                "complexity_max": max((item["complexity_max"] for item in members), default=0),
                "uncovered_lines": executable - covered,
                "line_rate": round(covered / executable, 4) if executable else None,
                "branch_rate": round(branches_covered / branches, 4) if branches else None,
                "commits": sum(item["history"]["commits"] for item in members),
                "lines_changed": sum(item["history"]["lines_changed"] for item in members),
                "effect_sites": sum(item["effect_sites"] for item in members),
                "attention_max": max(item["attention"]["value"] for item in members),
                "attention_mean": round(sum(item["attention"]["value"] for item in members) / len(members), 4),
            }
        )
    return result


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("root", type=Path, help="Repository root")
    parser.add_argument("--source", type=Path, required=True, help="Python source subtree, relative to root")
    parser.add_argument("--coverage", type=Path, help="coverage.py XML report, relative to root")
    parser.add_argument("--since", default="18 months ago", help="Git history window")
    parser.add_argument("--pretty", action="store_true")
    args = parser.parse_args()

    root = args.root.resolve()
    source_root = (root / args.source).resolve()
    coverage_path = (root / args.coverage).resolve() if args.coverage else None
    coverage, coverage_metadata = parse_coverage(coverage_path)
    history = parse_git_history(root, args.since)
    files = [
        collect_file(path, root, coverage, history)
        for path in sorted(source_root.rglob("*.py"))
        if "__pycache__" not in path.parts
    ]
    add_coupling(files)
    add_attention(files)
    payload = {
        "schema": "vibecodemap.python-quality/0.1",
        "repository": str(root),
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "scope": {"source": args.source.as_posix(), "git_since": args.since},
        "evidence": {
            "ast": {"method": "deterministic", "limitations": "Python-only; dynamic dispatch is unresolved."},
            "coverage": coverage_metadata,
            "git": {"method": "git numstat", "window": args.since},
        },
        "attention_formula": {
            "meaning": "relative review priority, not code quality",
            "normalization": "within-scan percentile mid-rank",
            "weights": {"complexity": 0.26, "uncovered": 0.24, "churn": 0.22, "effects": 0.16, "coupling": 0.12},
        },
        "groups": aggregate_groups(files),
        "files": files,
    }
    print(json.dumps(payload, indent=2 if args.pretty else None, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
