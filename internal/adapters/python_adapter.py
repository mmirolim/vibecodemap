#!/usr/bin/env python3
"""VCM Python AST adapter.

The Go orchestrator sends one centrally scoped AnalyzeRequest on stdin. This
adapter never walks the repository: it reads only listed Python files and
emits one versioned EvidenceEvent JSON object per line on stdout.
"""

from __future__ import annotations

import ast
from collections import Counter
import hashlib
import json
from pathlib import Path
import sys
from typing import Any, Iterable


EVENT_SCHEMA = "vibecodemap.evidence-event/0.1"
PRODUCER = "python-ast-v0"
READ_CALLS = {
    "db.session.get", "db.session.execute", "db.session.scalar",
    "db.session.scalars", "db.session.query", ".read_text", ".read_bytes",
}
WRITE_CALLS = {
    "db.session.add", "db.session.add_all", "db.session.delete",
    "db.session.flush", "db.session.commit", "db.session.rollback",
    ".write_text", ".write_bytes", ".save",
}
FILE_MUTATIONS = {
    "os.remove", "os.unlink", "os.rename", "os.replace", "os.makedirs",
    "shutil.copy", "shutil.copy2", "shutil.move", "shutil.rmtree",
    ".unlink", ".mkdir",
}
NETWORK_PREFIXES = ("requests.", "httpx.", "urllib.request.", "aiohttp.", "boto3.")
PROCESS_PREFIXES = ("subprocess.", "os.system", "os.popen")


def dotted_name(node: ast.AST) -> str | None:
    if isinstance(node, ast.Name):
        return node.id
    if isinstance(node, ast.Attribute):
        parent = dotted_name(node.value)
        return f"{parent}.{node.attr}" if parent else node.attr
    return None


def literal(node: ast.AST) -> Any:
    try:
        return ast.literal_eval(node)
    except (ValueError, TypeError):
        return None


def matches(name: str, candidates: Iterable[str]) -> bool:
    return any(name == candidate or name.endswith(candidate) for candidate in candidates)


def classify_call(node: ast.Call, name: str) -> set[str]:
    effects: set[str] = set()
    if name == "open":
        mode = "r"
        if len(node.args) > 1:
            mode = literal(node.args[1]) or mode
        for keyword in node.keywords:
            if keyword.arg == "mode":
                mode = literal(keyword.value) or mode
        effects.add("filesystem.write" if any(flag in str(mode) for flag in "wax+") else "filesystem.read")
    if matches(name, READ_CALLS):
        effects.add("database.read" if "db.session" in name or ".query" in name else "filesystem.read")
    if matches(name, WRITE_CALLS):
        effects.add("database.write" if "db.session" in name else "filesystem.write")
    if matches(name, FILE_MUTATIONS):
        effects.add("filesystem.mutate")
    if ".query." in name or name.endswith(".query"):
        effects.add("database.read")
    if name.startswith(NETWORK_PREFIXES):
        effects.add("network.call")
    if name.startswith(PROCESS_PREFIXES):
        effects.add("process.spawn")
    if name.startswith("logging.") or ".logger." in name or name.startswith("logger."):
        effects.add("telemetry.log")
    if any(marker in name for marker in ("generate_content", "generate_images")):
        effects.add("network.ai_provider")
    if any(marker in name for marker in ("upload_file", "download_blob", "upload_from_")):
        effects.add("network.object_storage")
    return effects


class FunctionSignals(ast.NodeVisitor):
    def __init__(self) -> None:
        self.complexity = 1
        self.depth = 0
        self.max_nesting = 0
        self.calls: set[str] = set()
        self.effects: Counter[str] = Counter()
        self.awaits = False

    def visit_FunctionDef(self, node: ast.FunctionDef) -> None:  # noqa: N802
        return

    def visit_AsyncFunctionDef(self, node: ast.AsyncFunctionDef) -> None:  # noqa: N802
        return

    def visit_ClassDef(self, node: ast.ClassDef) -> None:  # noqa: N802
        return

    def _nested(self, node: ast.AST, increment: int = 1) -> None:
        self.complexity += increment
        self.depth += 1
        self.max_nesting = max(self.max_nesting, self.depth)
        self.generic_visit(node)
        self.depth -= 1

    def visit_If(self, node: ast.If) -> None: self._nested(node)  # noqa: N802,E701
    def visit_For(self, node: ast.For) -> None: self._nested(node)  # noqa: N802,E701
    def visit_AsyncFor(self, node: ast.AsyncFor) -> None: self._nested(node)  # noqa: N802,E701
    def visit_While(self, node: ast.While) -> None: self._nested(node)  # noqa: N802,E701
    def visit_With(self, node: ast.With) -> None: self._nested(node)  # noqa: N802,E701
    def visit_AsyncWith(self, node: ast.AsyncWith) -> None: self._nested(node)  # noqa: N802,E701
    def visit_ExceptHandler(self, node: ast.ExceptHandler) -> None: self._nested(node)  # noqa: N802,E701
    def visit_IfExp(self, node: ast.IfExp) -> None: self._nested(node)  # noqa: N802,E701

    def visit_Assert(self, node: ast.Assert) -> None:  # noqa: N802
        self.complexity += 1
        self.generic_visit(node)

    def visit_Match(self, node: ast.Match) -> None:  # noqa: N802
        self._nested(node, max(1, len(node.cases)))

    def visit_BoolOp(self, node: ast.BoolOp) -> None:  # noqa: N802
        self.complexity += max(0, len(node.values) - 1)
        self.generic_visit(node)

    def visit_comprehension(self, node: ast.comprehension) -> None:
        self.complexity += 1 + len(node.ifs)
        self.generic_visit(node)

    def visit_Call(self, node: ast.Call) -> None:  # noqa: N802
        name = dotted_name(node.func)
        if name:
            self.calls.add(name)
            self.effects.update(classify_call(node, name))
        self.generic_visit(node)

    def visit_Await(self, node: ast.Await) -> None:  # noqa: N802
        self.awaits = True
        self.generic_visit(node)


def decorator_facts(decorator: ast.AST) -> dict[str, Any]:
    if not isinstance(decorator, ast.Call):
        return {"name": dotted_name(decorator) or ast.unparse(decorator)}
    result: dict[str, Any] = {"name": dotted_name(decorator.func) or ast.unparse(decorator.func)}
    if decorator.args:
        value = literal(decorator.args[0])
        if value is not None:
            result["argument"] = value
    for keyword in decorator.keywords:
        value = literal(keyword.value)
        if value is not None:
            result[keyword.arg or "kwargs"] = value
    return result


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


def imports_for(tree: ast.AST) -> list[str]:
    imports: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            imports.update(alias.name for alias in node.names)
        elif isinstance(node, ast.ImportFrom):
            module = node.module or ""
            imports.update(f"{module}.{alias.name}".strip(".") for alias in node.names)
    return sorted(imports)


def module_doc_line(tree: ast.Module) -> str:
    """Return a compact module docstring without importing inspect via ast.get_docstring."""
    if not tree.body:
        return ""
    first = tree.body[0]
    if not isinstance(first, ast.Expr) or not isinstance(first.value, ast.Constant):
        return ""
    if not isinstance(first.value.value, str):
        return ""
    lines = first.value.value.strip().splitlines()
    return lines[0].strip() if lines else ""


def routes_for(symbol: dict[str, Any]) -> list[dict[str, Any]]:
    routes: list[dict[str, Any]] = []
    for decorator in symbol["decorators"]:
        name = decorator["name"]
        if name.endswith((".route", ".get", ".post", ".put", ".patch", ".delete")):
            methods = decorator.get("methods")
            if not methods:
                methods = [name.rsplit(".", 1)[-1].upper()] if not name.endswith(".route") else ["GET"]
            routes.append({"path": decorator.get("argument", "<dynamic>"), "methods": methods,
                           "handler": symbol["name"], "line": symbol["line"]})
    return routes


def analyze_file(root: Path, relative: str) -> tuple[dict[str, Any], int]:
    candidate = (root / relative).resolve()
    try:
        candidate.relative_to(root)
    except ValueError as exc:
        raise ValueError(f"scoped path escapes repository root: {relative}") from exc
    source = candidate.read_text(encoding="utf-8")
    lines = source.splitlines()
    try:
        tree = ast.parse(source, filename=str(candidate))
    except SyntaxError as exc:
        return {"path": relative, "language": "python", "parse_error": {
            "line": exc.lineno, "column": exc.offset, "message": exc.msg,
        }}, len(lines)

    symbols: list[dict[str, Any]] = []
    effects: Counter[str] = Counter()
    routes: list[dict[str, Any]] = []
    for name, node in iter_functions(tree):
        visitor = FunctionSignals()
        for statement in node.body:
            visitor.visit(statement)
        effects.update(visitor.effects)
        symbol = {
            "name": name, "kind": "function", "line": node.lineno,
            "end_line": getattr(node, "end_lineno", node.lineno),
            "execution": "async" if isinstance(node, ast.AsyncFunctionDef) or visitor.awaits else "sync",
            "complexity": visitor.complexity, "max_nesting": visitor.max_nesting,
            "calls": sorted(visitor.calls), "effects": dict(sorted(visitor.effects.items())),
            "decorators": [decorator_facts(item) for item in node.decorator_list],
        }
        symbols.append(symbol)
        routes.extend(routes_for(symbol))
    return {
        "path": relative, "language": "python", "lines": len(lines),
        "code_lines": sum(1 for line in lines if line.strip() and not line.lstrip().startswith("#")),
        "module_doc": module_doc_line(tree),
        "imports": imports_for(tree), "symbols": symbols, "routes": routes,
        "quality": {
            "function_count": len(symbols),
            "max_function_lines": max((item["end_line"] - item["line"] + 1 for item in symbols), default=0),
            "complexity_total": sum(item["complexity"] for item in symbols),
            "complexity_max": max((item["complexity"] for item in symbols), default=0),
            "nesting_max": max((item["max_nesting"] for item in symbols), default=0),
            "effects": dict(sorted(effects.items())), "effect_sites": sum(effects.values()),
        },
        "limitations": ["Static Python AST only", "Dynamic imports and dispatch are unresolved",
                        "Calls and effects are candidates, not observed runtime behavior"],
    }, len(lines)


def event_id(path: str) -> str:
    return "python.file." + hashlib.sha256(path.encode("utf-8")).hexdigest()[:16]


def emit(event: dict[str, Any]) -> None:
    sys.stdout.write(json.dumps(event, separators=(",", ":"), ensure_ascii=False) + "\n")


def main() -> int:
    request = json.load(sys.stdin)
    if request.get("schema") != "vibecodemap.adapter-request/0.1":
        raise ValueError("unsupported adapter request schema")
    if request.get("adapter_id") != PRODUCER:
        raise ValueError("adapter request was sent to the wrong adapter")
    root = Path(request["root"]).resolve()
    for item in request.get("files", []):
        relative = item.get("path", "")
        if not relative.endswith(".py"):
            continue
        if item.get("action") == "summarize":
            payload = {"path": relative, "language": "python", "size_bytes": item.get("size_bytes", 0),
                       "summary_only": True, "reason": "central scope classified this file as summarize"}
            line_count = 0
        else:
            payload, line_count = analyze_file(root, relative)
        kind = "python.parse_error" if payload.get("parse_error") else "python.file_analysis"
        emit({"schema": EVENT_SCHEMA, "id": event_id(relative), "kind": kind,
              "subject": relative, "producer": PRODUCER, "confidence": 1.0,
              "source": {"path": relative, "line": 1, "end_line": line_count} if line_count else {"path": relative},
              "payload": payload})
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:  # Adapter failures belong on stderr, never in the event stream.
        print(f"python adapter: {exc}", file=sys.stderr)
        raise SystemExit(1) from exc
