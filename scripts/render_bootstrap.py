#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "PyYAML>=6.0,<7",
# ]
# ///
"""Render and validate tracked bootstrap manifests from repo conventions."""

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
    include_hooks: bool
    strip_hook_annotations: bool
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


COMPONENT_METADATA: dict[str, dict[str, Any]] = {
    "argocd": {
        "chart_ref": "oci://ghcr.io/argoproj/argo-helm/argo-cd",
        "chart_version": "9.5.2",
        "chart_digest": "sha256:91688034f0b2b52f022fb15c0e3ee4207244275039597debbbebc3c921f3e9aa",
        "release_name": "argocd",
        "namespace": "argocd",
        "create_namespace": True,
        "include_crds": True,
        "renders": {
            "bootstrap": {
                "tracked": True,
                "include_hooks": True,
                "strip_hook_annotations": True,
                "values": ("values/base.yaml", "values/bootstrap-overrides.yaml"),
                "output": "render/bootstrap.yaml",
            },
            "full": {
                "tracked": True,
                "include_hooks": True,
                "strip_hook_annotations": True,
                "values": ("values/base.yaml", "values/full-overrides.yaml"),
                "output": "render/full.yaml",
            },
        },
    },
    "cilium": {
        "chart_ref": "oci://quay.io/cilium/charts/cilium",
        "chart_version": "1.19.3",
        "chart_digest": "sha256:0683e1fc672e0c6587a8d8d43f6845430aaaed47e36dacbe77e3e621ea7a4c69",
        "release_name": "cilium",
        "namespace": "kube-system",
        "create_namespace": False,
        "include_crds": True,
        "renders": {
            "bootstrap": {
                "tracked": True,
                "include_hooks": False,
                "strip_hook_annotations": False,
                "values": ("values/base.yaml", "values/bootstrap-overrides.yaml"),
                "output": "render/bootstrap.yaml",
            },
            "full": {
                "tracked": False,
                "include_hooks": False,
                "strip_hook_annotations": False,
                "values": ("values/base.yaml", "values/full-overrides.yaml"),
                "output": ".state/render/full.yaml",
            },
        },
    },
    "kro": {
        "chart_ref": "oci://registry.k8s.io/kro/charts/kro",
        "chart_version": "0.9.1",
        "chart_digest": "sha256:37b00031550322ede84caa305024ada4dcc30cfe9fa3375822be2e212fa9411f",
        "release_name": "kro",
        "namespace": "kro-system",
        "create_namespace": True,
        "include_crds": True,
        "renders": {
            "full": {
                "tracked": True,
                "include_hooks": False,
                "strip_hook_annotations": False,
                "values": ("values/base.yaml", "values/full-overrides.yaml"),
                "output": "render/full.yaml",
            },
        },
    },
}


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
    for name, metadata in sorted(COMPONENT_METADATA.items()):
        directory = BOOTSTRAP_DIR / name
        if not directory.is_dir():
            raise BootstrapError(f"bootstrap component directory does not exist: {directory}")

        renders: dict[str, RenderLane] = {}
        for render_name, render_data in metadata["renders"].items():
            renders[render_name] = RenderLane(
                name=render_name,
                tracked=render_data["tracked"],
                include_hooks=render_data["include_hooks"],
                strip_hook_annotations=render_data["strip_hook_annotations"],
                values=tuple(directory / item for item in render_data["values"]),
                output=directory / render_data["output"],
            )

        components[name] = Component(
            name=name,
            directory=directory,
            chart_ref=metadata["chart_ref"],
            chart_version=metadata["chart_version"],
            chart_digest=metadata["chart_digest"],
            release_name=metadata["release_name"],
            namespace=metadata["namespace"],
            create_namespace=metadata["create_namespace"],
            include_crds=metadata["include_crds"],
            renders=renders,
        )

    return components


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
        "--skip-tests",
    ]
    if not lane.include_hooks:
        cmd.append("--no-hooks")
    if component.include_crds:
        cmd.append("--include-crds")
    for value_file in lane.values:
        cmd.extend(["--values", str(value_file)])

    rendered = run(cmd)

    try:
        documents = [doc for doc in yaml.safe_load_all(rendered) if doc is not None]
    except yaml.YAMLError as exc:
        raise BootstrapError(f"failed to parse rendered YAML for {component.name}/{lane.name}: {exc}") from exc

    if lane.strip_hook_annotations:
        documents = [strip_helm_hook_annotations(doc) for doc in documents]

    if component.create_namespace and not has_namespace_document(documents, component.namespace):
        documents.insert(0, namespace_document(component.namespace))

    header = [
        "# Generated by scripts/render_bootstrap.py; do not edit by hand.",
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


def strip_helm_hook_annotations(document: dict[str, Any]) -> dict[str, Any]:
    if not isinstance(document, dict):
        return document

    metadata = document.get("metadata")
    if not isinstance(metadata, dict):
        return document

    annotations = metadata.get("annotations")
    if not isinstance(annotations, dict):
        return document

    cleaned = {
        key: value
        for key, value in annotations.items()
        if key not in {"helm.sh/hook", "helm.sh/hook-delete-policy"}
    }
    if cleaned:
        metadata["annotations"] = cleaned
    else:
        metadata.pop("annotations", None)

    return document


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
