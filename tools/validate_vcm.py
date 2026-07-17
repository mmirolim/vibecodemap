#!/usr/bin/env python3
"""Validate a VibeCodeMap YAML/JSON model, including source references.

The JSON Schema validates serialization shape when the optional ``jsonschema``
package is available. The checks in this file validate graph references,
repository scope, source paths, and line ranges without third-party packages
beyond PyYAML.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any, Iterable

import yaml


ID_PATTERN = re.compile(r"^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$")
SOURCE_REF_ROLES = {
    "definition",
    "implementation",
    "configuration",
    "contract",
    "evidence",
    "template",
    "test",
    "generated",
    "schema",
}
EFFECT_TARGET_KINDS = {
    "data_store",
    "resource",
    "external_system",
    "operation",
    "component",
    "policy",
    "expected_component",
}
SOURCE_OPTIONAL_KINDS = {"actor", "external_system"}


class Validation:
    def __init__(self) -> None:
        self.errors: list[str] = []
        self.warnings: list[str] = []

    def error(self, message: str) -> None:
        self.errors.append(message)

    def warn(self, message: str) -> None:
        self.warnings.append(message)


def load_document(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        if path.suffix.lower() == ".json":
            value = json.load(handle)
        else:
            value = yaml.safe_load(handle)
    if not isinstance(value, dict):
        raise ValueError("document root must be a mapping")
    return value


def line_count(path: Path) -> int:
    return len(path.read_text(encoding="utf-8").splitlines())


def entity_groups(document: dict[str, Any]) -> Iterable[tuple[str, list[dict[str, Any]]]]:
    for name in ("artifacts", "elements", "relations", "flows", "findings", "views"):
        yield name, document.get(name, []) or []
    architecture = document.get("architecture", {}) or {}
    yield "architecture.styles", architecture.get("styles", []) or []
    yield "architecture.constraints", architecture.get("constraints", []) or []


def collect_source_refs(value: Any, location: str = "root") -> Iterable[tuple[str, dict[str, Any]]]:
    if isinstance(value, dict):
        if "artifact" in value and set(value).intersection({"lines", "symbol", "role", "supports"}):
            yield location, value
        for key, child in value.items():
            yield from collect_source_refs(child, f"{location}.{key}")
    elif isinstance(value, list):
        for index, child in enumerate(value):
            yield from collect_source_refs(child, f"{location}[{index}]")


def scoped_files(root: Path, scope: dict[str, Any]) -> set[str]:
    included: set[Path] = set()
    excluded: set[Path] = set()
    for pattern in scope.get("include", []) or []:
        included.update(item.resolve() for item in root.glob(pattern) if item.is_file())
    for pattern in scope.get("exclude", []) or []:
        excluded.update(item.resolve() for item in root.glob(pattern) if item.is_file())
    return {
        path.relative_to(root.resolve()).as_posix()
        for path in included - excluded
        if path.is_relative_to(root.resolve())
    }


def validate_shape(document: dict[str, Any], result: Validation) -> None:
    for key in ("vcm", "model", "scope", "artifacts", "elements"):
        if key not in document:
            result.error(f"missing required top-level field: {key}")
    if document.get("vcm") != "0.1":
        result.error(f"unsupported vcm version: {document.get('vcm')!r}")
    if not isinstance(document.get("artifacts", []), list):
        result.error("artifacts must be a list")
    if not isinstance(document.get("elements", []), list):
        result.error("elements must be a list")


def validate_ids(document: dict[str, Any], result: Validation) -> dict[str, tuple[str, dict[str, Any]]]:
    entities: dict[str, tuple[str, dict[str, Any]]] = {}
    for group, items in entity_groups(document):
        if not isinstance(items, list):
            result.error(f"{group} must be a list")
            continue
        for index, item in enumerate(items):
            location = f"{group}[{index}]"
            if not isinstance(item, dict):
                result.error(f"{location} must be a mapping")
                continue
            entity_id = item.get("id")
            if not isinstance(entity_id, str) or not ID_PATTERN.fullmatch(entity_id):
                result.error(f"{location}.id is invalid: {entity_id!r}")
                continue
            if entity_id in entities:
                previous = entities[entity_id][0]
                result.error(f"duplicate id {entity_id!r} at {location}; first seen at {previous}")
            else:
                entities[entity_id] = (location, item)
    return entities


def validate_artifacts(
    document: dict[str, Any],
    root: Path,
    result: Validation,
) -> tuple[dict[str, dict[str, Any]], dict[str, int]]:
    artifacts: dict[str, dict[str, Any]] = {}
    counts: dict[str, int] = {}
    paths_seen: dict[str, str] = {}

    for index, artifact in enumerate(document.get("artifacts", []) or []):
        if not isinstance(artifact, dict):
            continue
        location = f"artifacts[{index}]"
        artifact_id = artifact.get("id")
        path_value = artifact.get("path")
        summary = artifact.get("summary")
        if not isinstance(summary, str) or not summary.strip():
            result.error(f"{location}.summary must be non-empty")
        if not isinstance(path_value, str) or not path_value:
            result.error(f"{location}.path must be non-empty")
            continue
        relative = Path(path_value)
        if relative.is_absolute():
            result.error(f"{location}.path must be relative: {path_value}")
            continue
        resolved = (root / relative).resolve()
        if not resolved.is_relative_to(root):
            result.error(f"{location}.path escapes repository root: {path_value}")
            continue
        if not resolved.is_file():
            result.error(f"{location}.path does not exist: {path_value}")
            continue
        normalized = relative.as_posix()
        if normalized in paths_seen:
            result.error(
                f"artifact path {normalized!r} is duplicated by {artifact_id!r} and {paths_seen[normalized]!r}"
            )
        paths_seen[normalized] = str(artifact_id)
        actual_lines = line_count(resolved)
        counts[str(artifact_id)] = actual_lines
        declared_lines = (artifact.get("metrics") or {}).get("lines")
        if declared_lines is not None and declared_lines != actual_lines:
            result.error(
                f"{location}.metrics.lines is {declared_lines}, actual file has {actual_lines}: {path_value}"
            )
        if isinstance(artifact_id, str):
            artifacts[artifact_id] = artifact

    expected = scoped_files(root, document.get("scope", {}) or {})
    modeled = {artifact.get("path") for artifact in document.get("artifacts", []) if isinstance(artifact, dict)}
    missing = sorted(expected - modeled)
    extra = sorted(modeled - expected)
    for path in missing:
        result.error(f"scoped file has no artifact: {path}")
    for path in extra:
        result.warn(f"artifact is outside declared scope: {path}")
    return artifacts, counts


def validate_source_refs(
    document: dict[str, Any],
    artifacts: dict[str, dict[str, Any]],
    counts: dict[str, int],
    result: Validation,
) -> None:
    for location, source_ref in collect_source_refs(document):
        artifact_id = source_ref.get("artifact")
        if artifact_id not in artifacts:
            result.error(f"{location}.artifact does not resolve: {artifact_id!r}")
            continue
        role = source_ref.get("role")
        if role is not None and role not in SOURCE_REF_ROLES:
            result.error(f"{location}.role is invalid: {role!r}")
        lines = source_ref.get("lines")
        if lines is not None:
            if (
                not isinstance(lines, list)
                or len(lines) != 2
                or not all(isinstance(value, int) for value in lines)
            ):
                result.error(f"{location}.lines must be [start, end] integers")
                continue
            start, end = lines
            maximum = counts.get(str(artifact_id), 0)
            if start < 1 or end < start or end > maximum:
                result.error(
                    f"{location}.lines {lines} are outside 1..{maximum} for {artifacts[artifact_id]['path']}"
                )


def validate_elements(document: dict[str, Any], result: Validation) -> dict[str, dict[str, Any]]:
    elements = {
        item["id"]: item
        for item in document.get("elements", []) or []
        if isinstance(item, dict) and isinstance(item.get("id"), str)
    }
    for element_id, element in elements.items():
        parent = element.get("parent")
        if parent is not None and parent not in elements:
            result.error(f"element {element_id!r} parent does not resolve: {parent!r}")
        if parent == element_id:
            result.error(f"element {element_id!r} cannot parent itself")
        source_refs = element.get("source_refs", []) or []
        evidence = element.get("evidence", []) or []
        kind = element.get("kind")
        implementation = (element.get("reality") or {}).get("implementation")
        if kind not in SOURCE_OPTIONAL_KINDS and not source_refs and not evidence:
            result.error(f"internal element {element_id!r} has no source_refs or evidence")
        if implementation == "missing" and not source_refs and not evidence:
            result.error(f"missing element {element_id!r} must cite caller or requirement evidence")
        runtime = (element.get("reality") or {}).get("runtime")
        method = (element.get("generated") or {}).get("method")
        if runtime in {"observed", "contradicted"} and method != "runtime_observed":
            result.error(
                f"element {element_id!r} claims runtime={runtime!r} without runtime_observed provenance"
            )
    return elements


def validate_relations(
    document: dict[str, Any],
    elements: dict[str, dict[str, Any]],
    result: Validation,
) -> dict[str, dict[str, Any]]:
    relations: dict[str, dict[str, Any]] = {}
    for relation in document.get("relations", []) or []:
        if not isinstance(relation, dict) or not isinstance(relation.get("id"), str):
            continue
        relation_id = relation["id"]
        relations[relation_id] = relation
        for endpoint in ("from", "to"):
            value = relation.get(endpoint)
            if value not in elements:
                result.error(f"relation {relation_id!r} {endpoint} does not resolve: {value!r}")
        if not isinstance(relation.get("summary"), str) or not relation["summary"].strip():
            result.error(f"relation {relation_id!r} has no summary")
        if relation.get("effect") and relation.get("to") in elements:
            target_kind = elements[relation["to"]].get("kind")
            if target_kind not in EFFECT_TARGET_KINDS:
                result.error(
                    f"side-effect relation {relation_id!r} targets unsupported kind {target_kind!r}"
                )
        runtime = (relation.get("reality") or {}).get("runtime")
        method = (relation.get("generated") or {}).get("method")
        if runtime in {"observed", "contradicted"} and method != "runtime_observed":
            result.error(
                f"relation {relation_id!r} claims runtime={runtime!r} without runtime_observed provenance"
            )
    return relations


def validate_flows(
    document: dict[str, Any],
    elements: dict[str, dict[str, Any]],
    relations: dict[str, dict[str, Any]],
    result: Validation,
) -> None:
    for flow in document.get("flows", []) or []:
        if not isinstance(flow, dict):
            continue
        flow_id = flow.get("id", "<unknown>")
        if flow.get("trigger") not in elements:
            result.error(f"flow {flow_id!r} trigger does not resolve: {flow.get('trigger')!r}")
        steps = flow.get("steps", []) or []
        step_ids: set[str] = set()
        for index, step in enumerate(steps):
            step_id = step.get("id") if isinstance(step, dict) else None
            if not isinstance(step_id, str) or not ID_PATTERN.fullmatch(step_id):
                result.error(f"flow {flow_id!r} step {index} has invalid id: {step_id!r}")
            elif step_id in step_ids:
                result.error(f"flow {flow_id!r} has duplicate local step id: {step_id!r}")
            else:
                step_ids.add(step_id)
            relation_id = step.get("relation") if isinstance(step, dict) else None
            if relation_id not in relations:
                result.error(
                    f"flow {flow_id!r} step {step_id!r} relation does not resolve: {relation_id!r}"
                )
        for step in steps:
            if not isinstance(step, dict):
                continue
            for next_id in step.get("next", []) or []:
                if next_id not in step_ids:
                    result.error(
                        f"flow {flow_id!r} step {step.get('id')!r} next does not resolve: {next_id!r}"
                    )


def validate_other_references(
    document: dict[str, Any],
    elements: dict[str, dict[str, Any]],
    entities: dict[str, tuple[str, dict[str, Any]]],
    result: Validation,
) -> None:
    architecture = document.get("architecture", {}) or {}
    for style in architecture.get("styles", []) or []:
        if style.get("scope") not in elements:
            result.error(f"style {style.get('id')!r} scope does not resolve: {style.get('scope')!r}")
    for constraint in architecture.get("constraints", []) or []:
        if constraint.get("scope") not in elements:
            result.error(
                f"constraint {constraint.get('id')!r} scope does not resolve: {constraint.get('scope')!r}"
            )
    for finding in document.get("findings", []) or []:
        for subject in finding.get("subjects", []) or []:
            if subject not in entities:
                result.error(f"finding {finding.get('id')!r} subject does not resolve: {subject!r}")
    for view in document.get("views", []) or []:
        for root in view.get("roots", []) or []:
            if root not in elements:
                result.error(f"view {view.get('id')!r} root does not resolve: {root!r}")


def validate_provenance(document: dict[str, Any], result: Validation) -> None:
    for group, items in entity_groups(document):
        for index, item in enumerate(items):
            if not isinstance(item, dict):
                continue
            generated = item.get("generated")
            # Constraints and views are authored queries/rules and do not yet
            # require generation metadata in the 0.1 schema.
            if group in {"architecture.constraints", "views"}:
                continue
            if not isinstance(generated, dict):
                result.error(f"{group}[{index}] has no generated provenance")
                continue
            method = generated.get("method")
            confidence = generated.get("confidence")
            rationale = generated.get("rationale")
            if method not in {
                "deterministic",
                "ai_inferred",
                "runtime_observed",
                "human_declared",
                "human_confirmed",
            }:
                result.error(f"{group}[{index}].generated.method is invalid: {method!r}")
            if confidence not in {"high", "medium", "low"}:
                result.error(f"{group}[{index}].generated.confidence is invalid: {confidence!r}")
            if not isinstance(rationale, str) or not rationale.strip():
                result.error(f"{group}[{index}].generated.rationale must be non-empty")
            evidence = (item.get("source_refs") or []) + (item.get("evidence") or [])
            if method == "ai_inferred" and group not in {"artifacts", "flows"} and not evidence:
                result.error(f"AI-inferred {group}[{index}] has no source evidence")
            if confidence == "low" and not evidence:
                result.error(f"low-confidence {group}[{index}] has no source evidence")


def optional_json_schema_validation(
    document: dict[str, Any], schema_path: Path | None, result: Validation
) -> None:
    if schema_path is None:
        return
    schema = json.loads(schema_path.read_text(encoding="utf-8"))
    try:
        import jsonschema  # type: ignore
    except ImportError:
        result.warn("jsonschema package is not installed; semantic validation still ran")
        return
    validator = jsonschema.Draft202012Validator(schema)
    for error in sorted(validator.iter_errors(document), key=lambda item: list(item.path)):
        location = ".".join(str(part) for part in error.absolute_path) or "root"
        result.error(f"schema {location}: {error.message}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("model", type=Path)
    parser.add_argument("--schema", type=Path)
    args = parser.parse_args()

    document = load_document(args.model)
    result = Validation()
    validate_shape(document, result)

    root_value = ((document.get("model") or {}).get("repository") or {}).get("root")
    if not isinstance(root_value, str) or not root_value:
        result.error("model.repository.root must be a non-empty path")
        root = Path("/")
    else:
        root = Path(root_value).resolve()
        if not root.is_dir():
            result.error(f"repository root is not a directory: {root}")

    entities = validate_ids(document, result)
    artifacts, counts = validate_artifacts(document, root, result)
    validate_source_refs(document, artifacts, counts, result)
    elements = validate_elements(document, result)
    relations = validate_relations(document, elements, result)
    validate_flows(document, elements, relations, result)
    validate_other_references(document, elements, entities, result)
    validate_provenance(document, result)
    optional_json_schema_validation(document, args.schema, result)

    print(
        json.dumps(
            {
                "valid": not result.errors,
                "model": str(args.model.resolve()),
                "artifacts": len(document.get("artifacts", []) or []),
                "elements": len(document.get("elements", []) or []),
                "relations": len(document.get("relations", []) or []),
                "flows": len(document.get("flows", []) or []),
                "findings": len(document.get("findings", []) or []),
                "errors": result.errors,
                "warnings": result.warnings,
            },
            indent=2,
        )
    )
    return 0 if not result.errors else 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, yaml.YAMLError, json.JSONDecodeError) as exc:
        print(json.dumps({"valid": False, "fatal": str(exc)}, indent=2))
        raise SystemExit(2) from exc
