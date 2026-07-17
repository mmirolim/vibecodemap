#!/usr/bin/env python3
"""Extract conservative, source-linked facts from a Python codebase.

This is intentionally not an architecture detector. It gives an AI mapper
verifiable facts (files, symbols, routes, imports, and likely side effects)
that can be cited when producing a VibeCodeMap model.
"""

from __future__ import annotations

import argparse
import ast
import json
from pathlib import Path
from typing import Any, Iterable


DB_SESSION_READ_CALLS = {
    "db.session.get",
    "db.session.execute",
    "db.session.scalar",
    "db.session.scalars",
    "db.session.query",
}
DB_WRITE_CALLS = {
    "db.session.add",
    "db.session.add_all",
    "db.session.delete",
    "db.session.flush",
    "db.session.commit",
    "db.session.rollback",
    "db.session.execute",
}
FILE_READ_CALLS = {
    "Path.read_text",
    "Path.read_bytes",
    ".read_text",
    ".read_bytes",
}
FILE_WRITE_CALLS = {
    "Path.write_text",
    "Path.write_bytes",
    ".write_text",
    ".write_bytes",
    ".save",
}
FILE_MUTATION_CALLS = {
    "os.remove",
    "os.unlink",
    "os.rename",
    "os.replace",
    "os.makedirs",
    "shutil.copy",
    "shutil.copy2",
    "shutil.move",
    "shutil.rmtree",
    ".unlink",
    ".mkdir",
}
NETWORK_PREFIXES = (
    "requests.",
    "httpx.",
    "urllib.request.",
    "aiohttp.",
    "boto3.",
)
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


def call_matches(name: str, candidates: Iterable[str]) -> bool:
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
        if any(flag in str(mode) for flag in ("w", "a", "x", "+")):
            effects.add("filesystem.write")
        else:
            effects.add("filesystem.read")

    if call_matches(name, FILE_READ_CALLS):
        effects.add("filesystem.read")
    if call_matches(name, FILE_WRITE_CALLS):
        effects.add("filesystem.write")
    if call_matches(name, FILE_MUTATION_CALLS):
        effects.add("filesystem.mutate")

    if name in DB_WRITE_CALLS:
        operation = name.rsplit(".", 1)[-1]
        effects.add(f"database.{operation}")
    elif name in DB_SESSION_READ_CALLS or ".query." in name or name.endswith(".query"):
        effects.add("database.read")

    if name.startswith(NETWORK_PREFIXES):
        effects.add("network.call")
    if name.startswith(PROCESS_PREFIXES):
        effects.add("process.spawn")
    if name.startswith("logging.") or ".logger." in name or name.startswith("logger."):
        effects.add("telemetry.log")

    # SDK-specific outbound calls. These are conservative signals, not proof
    # of a particular provider or protocol.
    if any(marker in name for marker in ("generate_content", "generate_images")):
        effects.add("network.ai_provider")
    if any(marker in name for marker in ("upload_file", "download_blob", "upload_from_")):
        effects.add("network.object_storage")

    return effects


class BodyFacts(ast.NodeVisitor):
    def __init__(self) -> None:
        self.calls: set[str] = set()
        self.effects: set[str] = set()
        self.awaits = False

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
    if isinstance(decorator, ast.Call):
        name = dotted_name(decorator.func) or ast.unparse(decorator.func)
        result: dict[str, Any] = {"name": name}
        if decorator.args:
            first = literal(decorator.args[0])
            if first is not None:
                result["argument"] = first
        for keyword in decorator.keywords:
            value = literal(keyword.value)
            if value is not None:
                result[keyword.arg or "kwargs"] = value
        return result
    return {"name": dotted_name(decorator) or ast.unparse(decorator)}


def symbol_facts(
    node: ast.ClassDef | ast.FunctionDef | ast.AsyncFunctionDef,
    *,
    parent: str | None = None,
) -> dict[str, Any]:
    qualified_name = f"{parent}.{node.name}" if parent else node.name
    result: dict[str, Any] = {
        "name": node.name,
        "qualified_name": qualified_name,
        "kind": "class" if isinstance(node, ast.ClassDef) else "function",
        "line": node.lineno,
        "end_line": getattr(node, "end_lineno", node.lineno),
        "doc": (ast.get_docstring(node) or "").split("\n", 1)[0],
        "decorators": [decorator_facts(item) for item in node.decorator_list],
    }

    body = BodyFacts()
    body.visit(node)
    result["execution"] = "async" if isinstance(node, ast.AsyncFunctionDef) or body.awaits else "sync"
    result["effects"] = sorted(body.effects)
    result["calls"] = sorted(body.calls)

    if isinstance(node, ast.ClassDef):
        result["bases"] = [dotted_name(base) or ast.unparse(base) for base in node.bases]
        result["members"] = [
            symbol_facts(item, parent=qualified_name)
            for item in node.body
            if isinstance(item, (ast.FunctionDef, ast.AsyncFunctionDef))
        ]
    return result


def route_facts(symbol: dict[str, Any]) -> list[dict[str, Any]]:
    routes: list[dict[str, Any]] = []
    for decorator in symbol.get("decorators", []):
        name = decorator["name"]
        if name.endswith((".route", ".get", ".post", ".put", ".patch", ".delete")):
            methods = decorator.get("methods")
            if not methods:
                methods = [name.rsplit(".", 1)[-1].upper()] if not name.endswith(".route") else ["GET"]
            routes.append(
                {
                    "path": decorator.get("argument", "<dynamic>"),
                    "methods": methods,
                    "handler": symbol["qualified_name"],
                    "line": symbol["line"],
                }
            )
    return routes


def parse_file(path: Path, root: Path) -> dict[str, Any]:
    relative = path.relative_to(root).as_posix()
    text = path.read_text(encoding="utf-8")
    result: dict[str, Any] = {
        "path": relative,
        "language": "python",
        "lines": len(text.splitlines()),
        "module_doc": "",
        "imports": [],
        "symbols": [],
        "routes": [],
        "parse_error": None,
    }
    try:
        tree = ast.parse(text, filename=str(path))
    except SyntaxError as exc:
        result["parse_error"] = {"line": exc.lineno, "message": exc.msg}
        return result

    result["module_doc"] = (ast.get_docstring(tree) or "").split("\n", 1)[0]
    imports: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            imports.update(alias.name for alias in node.names)
        elif isinstance(node, ast.ImportFrom):
            module = node.module or ""
            imports.update(f"{module}.{alias.name}".strip(".") for alias in node.names)
    for node in tree.body:
        if isinstance(node, (ast.ClassDef, ast.FunctionDef, ast.AsyncFunctionDef)):
            symbol = symbol_facts(node)
            result["symbols"].append(symbol)
            result["routes"].extend(route_facts(symbol))
    result["imports"] = sorted(imports)
    return result


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("root", type=Path, help="Python source root")
    parser.add_argument("--pretty", action="store_true")
    args = parser.parse_args()

    root = args.root.resolve()
    files = [
        parse_file(path, root)
        for path in sorted(root.rglob("*.py"))
        if "__pycache__" not in path.parts
    ]
    payload = {
        "schema": "vibecodemap.python-facts/0.1",
        "root": str(root),
        "files": files,
    }
    print(json.dumps(payload, indent=2 if args.pretty else None, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
