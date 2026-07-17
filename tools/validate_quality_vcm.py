#!/usr/bin/env python3
"""Validate VCM quality-extension references and semantic invariants."""

from __future__ import annotations

import argparse
from collections import Counter
from pathlib import Path
from typing import Any, Iterable

import yaml


VALID_STATUSES = {"observed", "unknown", "not_applicable", "stale", "invalid"}


def load_yaml(path: Path) -> dict[str, Any]:
    value = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{path}: expected a YAML mapping")
    return value


def duplicate_ids(records: Iterable[dict[str, Any]]) -> set[str]:
    ids = [record.get("id") for record in records if record.get("id")]
    return {identifier for identifier, count in Counter(ids).items() if count > 1}


def core_ids(core: dict[str, Any]) -> tuple[set[str], set[str]]:
    identifiers: set[str] = set()
    artifacts = {item["id"] for item in core.get("artifacts", [])}
    identifiers.update(artifacts)
    if core.get("model", {}).get("id"):
        identifiers.add(core["model"]["id"])
    for section in ("elements", "relations", "flows", "findings", "views"):
        identifiers.update(item["id"] for item in core.get(section, []) if item.get("id"))
    architecture = core.get("architecture", {})
    for section in ("styles", "constraints"):
        identifiers.update(item["id"] for item in architecture.get(section, []) if item.get("id"))
    return identifiers, artifacts


def source_ref_errors(
    refs: Iterable[dict[str, Any]],
    *,
    path: str,
    artifacts: set[str],
) -> list[str]:
    errors: list[str] = []
    for index, ref in enumerate(refs):
        artifact = ref.get("artifact")
        if artifacts and artifact not in artifacts:
            errors.append(f"{path}[{index}].artifact does not resolve: {artifact}")
        lines = ref.get("lines")
        if lines and (len(lines) != 2 or lines[0] < 1 or lines[1] < lines[0]):
            errors.append(f"{path}[{index}].lines is not a valid inclusive range: {lines}")
    return errors


def validate(quality: dict[str, Any], core: dict[str, Any] | None) -> tuple[list[str], list[str]]:
    errors: list[str] = []
    warnings: list[str] = []
    if quality.get("vcm_quality") != "0.2-draft":
        errors.append("vcm_quality must equal 0.2-draft")

    core_identifiers, artifacts = core_ids(core or {})
    if core:
        expected = quality.get("model", {}).get("core_model")
        actual = core.get("model", {}).get("id")
        if expected != actual:
            errors.append(f"model.core_model is {expected}, but core model id is {actual}")

    collections = {
        "metric_catalog": quality.get("metric_catalog", []),
        "measurements": quality.get("measurements", []),
        "analyzer_findings": quality.get("analyzer_findings", []),
        "quality_findings": quality.get("quality_findings", []),
        "priority_models": quality.get("priority_models", []),
        "layout_profiles": quality.get("layout_profiles", []),
        "lenses": quality.get("lenses", []),
    }
    for name, records in collections.items():
        for duplicate in sorted(duplicate_ids(records)):
            errors.append(f"duplicate {name} id: {duplicate}")

    metric_ids = {item.get("id") for item in collections["metric_catalog"]}
    measurement_ids = {item.get("id") for item in collections["measurements"]}
    priority_model_ids = {item.get("id") for item in collections["priority_models"]}
    layout_ids = {item.get("id") for item in collections["layout_profiles"]}

    for index, metric in enumerate(collections["metric_catalog"]):
        default = metric.get("aggregation", {}).get("default")
        allowed = metric.get("aggregation", {}).get("allowed", [])
        if default not in allowed:
            errors.append(f"metric_catalog[{index}] default aggregation {default} is not allowed")

    for index, measurement in enumerate(collections["measurements"]):
        prefix = f"measurements[{index}]"
        metric = measurement.get("metric")
        if metric not in metric_ids:
            errors.append(f"{prefix}.metric does not resolve: {metric}")
        subject = measurement.get("subject")
        if core_identifiers and subject not in core_identifiers:
            errors.append(f"{prefix}.subject does not resolve in the core model: {subject}")
        status = measurement.get("status")
        if status not in VALID_STATUSES:
            errors.append(f"{prefix}.status is invalid: {status}")
        if status in {"observed", "stale"} and "value" not in measurement:
            errors.append(f"{prefix} with status {status} must contain value")
        if status in {"unknown", "not_applicable", "invalid"} and "value" in measurement:
            warnings.append(f"{prefix} has status {status} but also contains value")
        provenance = measurement.get("provenance", {})
        for required in ("producer", "method", "scope"):
            if not provenance.get(required):
                errors.append(f"{prefix}.provenance.{required} is required")
        if provenance.get("method") == "ai_inferred" and "value" in measurement:
            errors.append(f"{prefix} is a numeric measurement produced by ai_inferred")
        errors.extend(source_ref_errors(measurement.get("source_refs", []), path=f"{prefix}.source_refs", artifacts=artifacts))

    for index, finding in enumerate(collections["quality_findings"]):
        prefix = f"quality_findings[{index}]"
        for subject in finding.get("subjects", []):
            if core_identifiers and subject not in core_identifiers:
                errors.append(f"{prefix}.subjects does not resolve: {subject}")
        for factor in finding.get("factors", []):
            if factor not in measurement_ids:
                errors.append(f"{prefix}.factors does not resolve: {factor}")
        generated = finding.get("generated", {})
        if generated.get("method") == "ai_inferred" and not generated.get("rationale"):
            errors.append(f"{prefix} is AI-inferred but has no rationale")
        errors.extend(source_ref_errors(finding.get("evidence", []), path=f"{prefix}.evidence", artifacts=artifacts))

    for index, finding in enumerate(collections["analyzer_findings"]):
        prefix = f"analyzer_findings[{index}]"
        subject = finding.get("subject")
        if core_identifiers and subject not in core_identifiers:
            errors.append(f"{prefix}.subject does not resolve: {subject}")
        errors.extend(source_ref_errors(finding.get("source_refs", []), path=f"{prefix}.source_refs", artifacts=artifacts))

    for index, model in enumerate(collections["priority_models"]):
        prefix = f"priority_models[{index}]"
        total = 0.0
        for factor in model.get("factors", []):
            metric = factor.get("metric")
            if metric not in metric_ids:
                errors.append(f"{prefix}.factors metric does not resolve: {metric}")
            total += float(factor.get("weight", 0))
        if abs(total - 1.0) > 1e-9:
            errors.append(f"{prefix}.factors weights sum to {total:.6f}, expected 1.0")

    for index, result in enumerate(quality.get("priority_results", [])):
        prefix = f"priority_results[{index}]"
        if result.get("model") not in priority_model_ids:
            errors.append(f"{prefix}.model does not resolve: {result.get('model')}")
        subject = result.get("subject")
        if core_identifiers and subject not in core_identifiers:
            errors.append(f"{prefix}.subject does not resolve: {subject}")

    for index, layout in enumerate(collections["layout_profiles"]):
        prefix = f"layout_profiles[{index}]"
        for field in ("footprint", "height"):
            metric = layout.get(field)
            if metric not in metric_ids:
                errors.append(f"{prefix}.{field} metric does not resolve: {metric}")
        effect_metric = layout.get("effect_contact", {}).get("metric")
        if effect_metric and effect_metric not in metric_ids:
            errors.append(f"{prefix}.effect_contact.metric does not resolve: {effect_metric}")

    for index, lens in enumerate(collections["lenses"]):
        prefix = f"lenses[{index}]"
        if lens.get("layout") not in layout_ids:
            errors.append(f"{prefix}.layout does not resolve: {lens.get('layout')}")
        heat_metric = lens.get("heat", {}).get("metric")
        if heat_metric and heat_metric not in metric_ids:
            errors.append(f"{prefix}.heat.metric does not resolve: {heat_metric}")
        if lens.get("heat"):
            legend = lens.get("legend")
            if not isinstance(legend, dict):
                errors.append(f"{prefix}.legend is required when heat is encoded")
            else:
                for field in ("name", "low_label", "high_label"):
                    if not legend.get(field):
                        errors.append(f"{prefix}.legend.{field} is required")

    if not any(measurement.get("status") in {"unknown", "stale"} for measurement in collections["measurements"]):
        warnings.append("no unknown or stale evidence is represented; verify that absence was not coerced to zero")
    return errors, warnings


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("quality", type=Path)
    parser.add_argument("--core", type=Path)
    args = parser.parse_args()

    quality = load_yaml(args.quality)
    core = load_yaml(args.core) if args.core else None
    errors, warnings = validate(quality, core)
    for warning in warnings:
        print(f"WARNING: {warning}")
    for error in errors:
        print(f"ERROR: {error}")
    print(f"quality validation: {len(errors)} error(s), {len(warnings)} warning(s)")
    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main())
