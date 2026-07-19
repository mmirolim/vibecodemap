#!/usr/bin/env python3
"""Validate a repository-owned VibeCodeMap project manifest.

JSON Schema checks serialization shape when ``jsonschema`` is installed. The
custom checks enforce cross-record identity, decomposition codes, correction
operations, input paths, and render-profile references.
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
SCOPE_ACTIONS = {"analyze", "summarize", "externalize", "ignore"}
GENERATED_ACTIONS = {"summarize", "externalize", "ignore"}


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
        value = json.load(handle) if path.suffix.lower() == ".json" else yaml.safe_load(handle)
    if not isinstance(value, dict):
        raise ValueError("document root must be a mapping")
    return value


def validate_schema(document: dict[str, Any], schema_path: Path, result: Validation) -> None:
    try:
        import jsonschema
    except ImportError:
        result.warn("jsonschema is unavailable; structural schema validation was skipped")
        return

    schema = json.loads(schema_path.read_text(encoding="utf-8"))
    validator = jsonschema.Draft202012Validator(schema)
    for error in sorted(validator.iter_errors(document), key=lambda item: list(item.absolute_path)):
        location = ".".join(str(part) for part in error.absolute_path) or "root"
        result.error(f"schema {location}: {error.message}")


def id_records(document: dict[str, Any]) -> Iterable[tuple[str, Any]]:
    analysis = document.get("analysis") or {}
    scope = analysis.get("scope") or {}
    generated = analysis.get("generated_detection") or {}
    for index, item in enumerate(scope.get("rules") or []):
        yield f"analysis.scope.rules[{index}]", item
    for index, item in enumerate(generated.get("markers") or []):
        yield f"analysis.generated_detection.markers[{index}]", item
    for decomposition_index, decomposition in enumerate(document.get("decompositions") or []):
        yield f"decompositions[{decomposition_index}]", decomposition
        if isinstance(decomposition, dict):
            for district_index, district in enumerate(decomposition.get("districts") or []):
                yield f"decompositions[{decomposition_index}].districts[{district_index}]", district
    for group in ("expectations", "corrections", "render_profiles"):
        for index, item in enumerate(document.get(group) or []):
            yield f"{group}[{index}]", item
            if group == "render_profiles" and isinstance(item, dict):
                for band_index, band in enumerate(((item.get("encodings") or {}).get("bands") or [])):
                    yield f"{group}[{index}].encodings.bands[{band_index}]", band
    for group in ("boundaries", "security_reviews"):
        for index, item in enumerate(document.get(group) or []):
            yield f"{group}[{index}]", item


def validate_ids(document: dict[str, Any], result: Validation) -> set[str]:
    seen: dict[str, str] = {}
    for location, item in id_records(document):
        if not isinstance(item, dict):
            continue
        value = item.get("id")
        if not isinstance(value, str) or not ID_PATTERN.fullmatch(value):
            result.error(f"{location}.id is invalid: {value!r}")
            continue
        if value in seen:
            result.error(f"duplicate id {value!r} at {location}; first seen at {seen[value]}")
        else:
            seen[value] = location
    return set(seen)


def validate_inputs(document: dict[str, Any], manifest_path: Path, result: Validation) -> None:
    project = document.get("project") or {}
    repository = project.get("repository") or {}
    root_value = repository.get("root")
    if isinstance(root_value, str) and root_value:
        root = Path(root_value)
        if not root.is_absolute():
            root = (manifest_path.parent / root).resolve()
        if not root.is_dir():
            result.warn(f"project.repository.root is not available locally: {root}")

    inputs = project.get("inputs") or {}
    paths: list[tuple[str, str]] = []
    for key in ("structural_model", "quality_model"):
        value = inputs.get(key)
        if isinstance(value, str) and value:
            paths.append((f"project.inputs.{key}", value))
    for key in ("runtime_models", "requirements"):
        for index, value in enumerate(inputs.get(key) or []):
            if isinstance(value, str) and value:
                paths.append((f"project.inputs.{key}[{index}]", value))

    for location, value in paths:
        path = Path(value)
        if not path.is_absolute():
            path = (manifest_path.parent / path).resolve()
        if not path.exists():
            result.warn(f"{location} does not resolve locally: {path}")


def validate_languages(document: dict[str, Any], result: Validation) -> None:
    languages = ((document.get("analysis") or {}).get("languages") or [])
    seen: set[str] = set()
    for index, profile in enumerate(languages):
        if not isinstance(profile, dict):
            continue
        language = profile.get("id")
        if language in seen:
            result.error(f"duplicate language profile {language!r} at analysis.languages[{index}]")
        elif isinstance(language, str):
            seen.add(language)
        if profile.get("enabled") and not profile.get("capabilities"):
            result.error(f"enabled language {language!r} declares no capabilities")


def validate_scope(document: dict[str, Any], result: Validation) -> None:
    analysis = document.get("analysis") or {}
    scope = analysis.get("scope") or {}
    if scope.get("default_action") not in SCOPE_ACTIONS:
        result.error(f"analysis.scope.default_action is invalid: {scope.get('default_action')!r}")
    for index, rule in enumerate(scope.get("rules") or []):
        if not isinstance(rule, dict):
            continue
        if rule.get("action") not in SCOPE_ACTIONS:
            result.error(
                f"analysis.scope.rules[{index}].action is invalid: {rule.get('action')!r}"
            )
        patterns = rule.get("patterns") or []
        if any("\\" in pattern for pattern in patterns if isinstance(pattern, str)):
            result.error(f"analysis.scope.rules[{index}] uses backslashes; globs must use forward slashes")

    generated = analysis.get("generated_detection") or {}
    if generated.get("default_action") not in GENERATED_ACTIONS:
        result.error(
            "analysis.generated_detection.default_action is invalid: "
            f"{generated.get('default_action')!r}"
        )
    for index, marker in enumerate(generated.get("markers") or []):
        if not isinstance(marker, dict):
            continue
        if marker.get("action") not in GENERATED_ACTIONS:
            result.error(
                "analysis.generated_detection.markers"
                f"[{index}].action is invalid: {marker.get('action')!r}"
            )
        expression = marker.get("regex")
        if isinstance(expression, str):
            try:
                re.compile(expression)
            except re.error as error:
                result.error(
                    f"analysis.generated_detection.markers[{index}].regex is invalid: {error}"
                )


def validate_decompositions(document: dict[str, Any], result: Validation) -> tuple[set[str], set[str]]:
    decomposition_ids: set[str] = set()
    district_ids: set[str] = set()
    for index, decomposition in enumerate(document.get("decompositions") or []):
        if not isinstance(decomposition, dict):
            continue
        decomposition_id = decomposition.get("id")
        if isinstance(decomposition_id, str):
            decomposition_ids.add(decomposition_id)
        codes: dict[str, str] = {}
        for district_index, district in enumerate(decomposition.get("districts") or []):
            if not isinstance(district, dict):
                continue
            location = f"decompositions[{index}].districts[{district_index}]"
            district_id = district.get("id")
            code = district.get("code")
            if isinstance(district_id, str):
                district_ids.add(district_id)
            if isinstance(code, str):
                if code in codes:
                    result.error(
                        f"district code {code!r} is duplicated in decomposition {decomposition_id!r}: "
                        f"{codes[code]} and {district_id}"
                    )
                else:
                    codes[code] = str(district_id)
            selector = district.get("members") or {}
            if not any(selector.get(key) for key in ("element_ids", "path_globs", "facets")):
                result.error(f"{location}.members has no positive selector")
    return decomposition_ids, district_ids


def validate_expectations(document: dict[str, Any], district_ids: set[str], result: Validation) -> None:
    for index, expectation in enumerate(document.get("expectations") or []):
        if not isinstance(expectation, dict):
            continue
        parent = expectation.get("parent")
        if isinstance(parent, str) and parent.startswith("district.") and parent not in district_ids:
            result.error(f"expectations[{index}].parent does not resolve to a district: {parent!r}")


def validate_corrections(document: dict[str, Any], result: Validation) -> None:
    for correction_index, correction in enumerate(document.get("corrections") or []):
        if not isinstance(correction, dict):
            continue
        for operation_index, operation in enumerate(correction.get("operations") or []):
            if not isinstance(operation, dict):
                continue
            location = f"corrections[{correction_index}].operations[{operation_index}]"
            action = operation.get("action")
            has_value = "value" in operation
            if action in {"set", "merge", "add"} and not has_value:
                result.error(f"{location} action {action!r} requires value")
            if action == "remove" and has_value:
                result.warn(f"{location} remove ignores its value")


def structural_ids(document: dict[str, Any], manifest_path: Path, result: Validation) -> set[str]:
    structural_model = (((document.get("project") or {}).get("inputs") or {}).get("structural_model"))
    if not isinstance(structural_model, str) or not structural_model:
        return set()
    path = Path(structural_model)
    if not path.is_absolute():
        path = (manifest_path.parent / path).resolve()
    try:
        model = load_document(path)
    except (OSError, ValueError, yaml.YAMLError, json.JSONDecodeError) as error:
        result.warn(f"could not load structural model for boundary references: {error}")
        return set()
    values: set[str] = set()
    for group in ("artifacts", "elements", "relations", "findings"):
        for item in model.get(group) or []:
            if isinstance(item, dict) and isinstance(item.get("id"), str):
                values.add(item["id"])
    return values


def validate_boundaries_and_security(
    document: dict[str, Any], model_ids: set[str], result: Validation
) -> None:
    for index, boundary in enumerate(document.get("boundaries") or []):
        if not isinstance(boundary, dict):
            continue
        subject = boundary.get("subject")
        if model_ids and subject not in model_ids:
            result.error(f"boundaries[{index}].subject does not resolve: {subject!r}")
        if boundary.get("direction") not in {"input", "output", "bidirectional"}:
            result.error(f"boundaries[{index}].direction is invalid: {boundary.get('direction')!r}")

    severities = {"info", "low", "medium", "high", "critical"}
    statuses = {"review_candidate", "confirmed", "accepted", "false_positive", "fixed"}
    for index, review in enumerate(document.get("security_reviews") or []):
        if not isinstance(review, dict):
            continue
        if review.get("severity") not in severities:
            result.error(f"security_reviews[{index}].severity is invalid: {review.get('severity')!r}")
        if review.get("status") not in statuses:
            result.error(f"security_reviews[{index}].status is invalid: {review.get('status')!r}")
        for subject_index, subject in enumerate(review.get("subjects") or []):
            if model_ids and subject not in model_ids:
                result.error(
                    f"security_reviews[{index}].subjects[{subject_index}] does not resolve: {subject!r}"
                )


def validate_render_profiles(
    document: dict[str, Any], decomposition_ids: set[str], result: Validation
) -> None:
    for index, profile in enumerate(document.get("render_profiles") or []):
        if not isinstance(profile, dict):
            continue
        decomposition = profile.get("decomposition")
        if decomposition not in decomposition_ids:
            result.error(
                f"render_profiles[{index}].decomposition does not resolve: {decomposition!r}"
            )
        bands = ((profile.get("encodings") or {}).get("bands") or [])
        orders: dict[int, str] = {}
        metrics: dict[str, str] = {}
        for band_index, band in enumerate(bands):
            if not isinstance(band, dict):
                continue
            location = f"render_profiles[{index}].encodings.bands[{band_index}]"
            order = band.get("order")
            metric = band.get("metric")
            band_id = str(band.get("id"))
            if isinstance(order, int):
                if order in orders:
                    result.error(f"{location}.order duplicates {orders[order]!r}: {order}")
                else:
                    orders[order] = band_id
            if isinstance(metric, str):
                if metric in metrics:
                    result.error(f"{location}.metric duplicates {metrics[metric]!r}: {metric}")
                else:
                    metrics[metric] = band_id
        if sorted(orders) != list(range(1, len(orders) + 1)):
            result.error(
                f"render_profiles[{index}] band orders must be contiguous from 1: {sorted(orders)}"
            )


def validate(document: dict[str, Any], manifest_path: Path, schema_path: Path) -> Validation:
    result = Validation()
    if document.get("vcm_project") != "0.1":
        result.error(f"unsupported vcm_project version: {document.get('vcm_project')!r}")
    validate_schema(document, schema_path, result)
    validate_ids(document, result)
    validate_inputs(document, manifest_path, result)
    validate_languages(document, result)
    validate_scope(document, result)
    decomposition_ids, district_ids = validate_decompositions(document, result)
    validate_expectations(document, district_ids, result)
    validate_corrections(document, result)
    model_ids = structural_ids(document, manifest_path, result)
    validate_boundaries_and_security(document, model_ids, result)
    validate_render_profiles(document, decomposition_ids, result)
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest", type=Path)
    parser.add_argument(
        "--schema",
        type=Path,
        default=Path(__file__).resolve().parents[1] / "dsl" / "vibecodemap-project-0.1.schema.json",
    )
    arguments = parser.parse_args()

    try:
        document = load_document(arguments.manifest)
        result = validate(document, arguments.manifest.resolve(), arguments.schema.resolve())
    except (OSError, ValueError, yaml.YAMLError, json.JSONDecodeError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 2

    for warning in result.warnings:
        print(f"warning: {warning}")
    for error in result.errors:
        print(f"error: {error}")
    if result.errors:
        print(f"invalid: {len(result.errors)} error(s), {len(result.warnings)} warning(s)")
        return 1
    print(f"valid: {len(result.warnings)} warning(s)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
