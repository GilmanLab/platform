#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "PyYAML>=6.0,<7",
# ]
# ///
"""Render and validate tracked bootstrap manifests."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
from pathlib import Path
import shutil
import subprocess
import sys
from typing import Any

import yaml


ROOT = Path(__file__).resolve().parents[1]
BOOTSTRAP_DIR = ROOT / "bootstrap"
REQUIRED_TOOLS = ("helm",)


class BootstrapError(RuntimeError):
    """Raised when bootstrap inputs or rendering steps are invalid."""


@dataclass(frozen=True)
class RenderLane:
    name: str
    tracked: bool
    values: tuple[Path, ...]
    output: Path


@dataclass(frozen=True)
class Component:
    name: str
    directory: Path
    chart_ref: str
    chart_version: str
    chart_digest: str
    release_name: str
    namespace: str
    create_namespace: bool
    include_crds: bool
    renders: dict[str, RenderLane]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    list_parser = subparsers.add_parser("list", help="Show configured bootstrap components")
    list_parser.add_argument("--component", help="Restrict output to one component")

    render_parser = subparsers.add_parser("render", help="Render bootstrap manifests")
    render_parser.add_argument("--component", help="Restrict output to one component")
    render_parser.add_argument(
        "--tracked-only",
        action="store_true",
        help="Render only tracked artifacts",
    )

    validate_parser = subparsers.add_parser(
        "validate", help="Verify tracked bootstrap manifests are current and safe to publish"
    )
    validate_parser.add_argument("--component", help="Restrict validation to one component")

    args = parser.parse_args()

    try:
        ensure_tools()
        components = load_components()
        selected = select_components(components, args.component)

        if args.command == "list":
            list_components(selected)
        elif args.command == "render":
            render_components(selected, tracked_only=args.tracked_only)
        elif args.command == "validate":
            validate_components(selected)
        else:
            raise BootstrapError(f"unsupported command: {args.command}")
    except BootstrapError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    return 0


def ensure_tools() -> None:
    missing = [tool for tool in REQUIRED_TOOLS if shutil.which(tool) is None]
    if missing:
        raise BootstrapError(f"missing required tool(s): {', '.join(missing)}")


def run(cmd: list[str]) -> str:
    try:
        result = subprocess.run(cmd, check=True, text=True, capture_output=True)
    except subprocess.CalledProcessError as exc:
        message = exc.stderr.strip() or exc.stdout.strip() or "command failed"
        raise BootstrapError(f"{' '.join(cmd)}: {message}") from exc

    return result.stdout


def load_components() -> dict[str, Component]:
    components: dict[str, Component] = {}
    for source_path in sorted(BOOTSTRAP_DIR.glob("*/source.yaml")):
        directory = source_path.parent
        data = load_yaml_file(source_path)

        chart = require_mapping(data, "chart", source_path)
        renders_raw = require_mapping(data, "renders", source_path)
        renders: dict[str, RenderLane] = {}
        for render_name, render_data in renders_raw.items():
            if not isinstance(render_name, str):
                raise BootstrapError(f"{source_path}: render key must be a string")
            if not isinstance(render_data, dict):
                raise BootstrapError(f"{source_path}: render {render_name!r} must be a mapping")

            values = render_data.get("values")
            if not isinstance(values, list) or not values or not all(isinstance(item, str) for item in values):
                raise BootstrapError(f"{source_path}: render {render_name!r} must define a non-empty values list")

            output = render_data.get("output")
            if not isinstance(output, str) or not output:
                raise BootstrapError(f"{source_path}: render {render_name!r} is missing output")

            tracked = render_data.get("tracked")
            if not isinstance(tracked, bool):
                raise BootstrapError(f"{source_path}: render {render_name!r} must define tracked as true/false")

            renders[render_name] = RenderLane(
                name=render_name,
                tracked=tracked,
                values=tuple(directory / item for item in values),
                output=directory / output,
            )

        component = Component(
            name=require_string(data, "name", source_path),
            directory=directory,
            chart_ref=require_string(chart, "ref", source_path),
            chart_version=require_string(chart, "version", source_path),
            chart_digest=require_string(chart, "digest", source_path),
            release_name=require_string(data, "releaseName", source_path),
            namespace=require_string(data, "namespace", source_path),
            create_namespace=require_bool(data, "createNamespace", source_path),
            include_crds=require_bool(data, "includeCRDs", source_path),
            renders=renders,
        )

        if component.name in components:
            raise BootstrapError(f"duplicate component name: {component.name}")
        components[component.name] = component

    if not components:
        raise BootstrapError(f"no bootstrap component definitions found under {BOOTSTRAP_DIR}")

    return components


def load_yaml_file(path: Path) -> dict[str, Any]:
    try:
        with path.open("r", encoding="utf-8") as fh:
            data = yaml.safe_load(fh)
    except OSError as exc:
        raise BootstrapError(f"failed to read {path}: {exc}") from exc
    except yaml.YAMLError as exc:
        raise BootstrapError(f"failed to parse YAML in {path}: {exc}") from exc

    if not isinstance(data, dict):
        raise BootstrapError(f"{path}: top-level YAML document must be a mapping")
    return data


def require_mapping(data: dict[str, Any], key: str, path: Path) -> dict[str, Any]:
    value = data.get(key)
    if not isinstance(value, dict):
        raise BootstrapError(f"{path}: missing or invalid mapping field {key!r}")
    return value


def require_string(data: dict[str, Any], key: str, path: Path) -> str:
    value = data.get(key)
    if not isinstance(value, str) or not value:
        raise BootstrapError(f"{path}: missing or invalid string field {key!r}")
    return value


def require_bool(data: dict[str, Any], key: str, path: Path) -> bool:
    value = data.get(key)
    if not isinstance(value, bool):
        raise BootstrapError(f"{path}: missing or invalid boolean field {key!r}")
    return value


def select_components(components: dict[str, Component], name: str | None) -> list[Component]:
    if name is None:
        return [components[key] for key in sorted(components)]
    if name not in components:
        raise BootstrapError(f"unknown component: {name}")
    return [components[name]]


def list_components(components: list[Component]) -> None:
    for component in components:
        print(
            f"{component.name}\t{component.chart_ref}@{component.chart_version}\t"
            f"{component.namespace}\t{component.chart_digest}"
        )
        for lane in component.renders.values():
            tracked = "tracked" if lane.tracked else "local"
            print(f"  - {lane.name}: {tracked} -> {lane.output.relative_to(component.directory)}")


def render_components(components: list[Component], *, tracked_only: bool) -> None:
    for component in components:
        for lane in component.renders.values():
            if tracked_only and not lane.tracked:
                continue
            text = render_lane_text(component, lane)
            lane.output.parent.mkdir(parents=True, exist_ok=True)
            lane.output.write_text(text, encoding="utf-8")
            print(f"rendered {component.name}/{lane.name}: {lane.output.relative_to(ROOT)}")


def validate_components(components: list[Component]) -> None:
    for component in components:
        for lane in component.renders.values():
            if not lane.tracked:
                continue

            expected = render_lane_text(component, lane)
            if not lane.output.is_file():
                raise BootstrapError(f"tracked render is missing: {lane.output}")

            current = lane.output.read_text(encoding="utf-8")
            if current != expected:
                raise BootstrapError(
                    f"tracked render is out of date: {lane.output.relative_to(ROOT)}; run `just render`"
                )

            documents = [doc for doc in yaml.safe_load_all(current) if doc is not None]
            if not documents:
                raise BootstrapError(f"tracked render is empty: {lane.output.relative_to(ROOT)}")

            if component.create_namespace and not has_namespace_document(documents, component.namespace):
                raise BootstrapError(
                    f"tracked render is missing Namespace/{component.namespace}: {lane.output.relative_to(ROOT)}"
                )

            leaked_secrets = secret_documents_with_material(documents)
            if leaked_secrets:
                names = ", ".join(leaked_secrets)
                raise BootstrapError(
                    f"tracked render embeds secret material ({names}): {lane.output.relative_to(ROOT)}"
                )

            print(f"validated {component.name}/{lane.name}: {lane.output.relative_to(ROOT)}")


def render_lane_text(component: Component, lane: RenderLane) -> str:
    for value_file in lane.values:
        if not value_file.is_file():
            raise BootstrapError(f"missing values file: {value_file}")

    cmd = [
        "helm",
        "template",
        component.release_name,
        component.chart_ref,
        "--version",
        component.chart_version,
        "--namespace",
        component.namespace,
        "--no-hooks",
        "--skip-tests",
    ]
    if component.include_crds:
        cmd.append("--include-crds")
    for value_file in lane.values:
        cmd.extend(["--values", str(value_file)])

    rendered = run(cmd)

    try:
        documents = [doc for doc in yaml.safe_load_all(rendered) if doc is not None]
    except yaml.YAMLError as exc:
        raise BootstrapError(f"failed to parse rendered YAML for {component.name}/{lane.name}: {exc}") from exc

    if component.create_namespace and not has_namespace_document(documents, component.namespace):
        documents.insert(0, namespace_document(component.namespace))

    header = [
        f"# Generated by scripts/render_bootstrap.py; do not edit by hand.",
        f"# Component: {component.name}",
        f"# Render lane: {lane.name}",
        f"# Source: {component.chart_ref}@{component.chart_version}",
        "",
    ]
    body = yaml.safe_dump_all(
        documents,
        explicit_start=True,
        sort_keys=False,
        default_flow_style=False,
    )
    return "\n".join(header) + body


def has_namespace_document(documents: list[dict[str, Any]], namespace: str) -> bool:
    for doc in documents:
        if not isinstance(doc, dict):
            continue
        if doc.get("kind") != "Namespace":
            continue
        metadata = doc.get("metadata") or {}
        if metadata.get("name") == namespace:
            return True
    return False


def namespace_document(namespace: str) -> dict[str, Any]:
    return {
        "apiVersion": "v1",
        "kind": "Namespace",
        "metadata": {
            "name": namespace,
        },
    }


def secret_documents_with_material(documents: list[dict[str, Any]]) -> list[str]:
    secret_names: list[str] = []
    for doc in documents:
        if not isinstance(doc, dict) or doc.get("kind") != "Secret":
            continue

        data = doc.get("data") or {}
        string_data = doc.get("stringData") or {}
        if data or string_data:
            metadata = doc.get("metadata") or {}
            secret_names.append(str(metadata.get("name", "<unnamed-secret>")))
    return secret_names


if __name__ == "__main__":
    raise SystemExit(main())
