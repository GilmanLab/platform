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
class Dependency:
    name: str
    alias: str
    repository: str
    version: str


@dataclass(frozen=True)
class Component:
    name: str
    directory: Path
    chart_path: Path
    wrapper_version: str
    app_version: str
    dependency: Dependency
    release_name: str
    namespace: str
    create_namespace: bool
    include_crds: bool
    renders: dict[str, RenderLane]


COMPONENT_METADATA: dict[str, dict[str, Any]] = {
    "argocd": {
        "release_name": "argocd",
        "namespace": "argocd",
        "create_namespace": True,
        "include_crds": True,
        "renders": {
            "bootstrap": {
                "tracked": True,
                "include_hooks": True,
                "strip_hook_annotations": True,
                "values": ("bootstrap-values.yaml",),
                "output": "render/bootstrap.yaml",
            },
            "full": {
                "tracked": True,
                "include_hooks": True,
                "strip_hook_annotations": True,
                "values": (),
                "output": "render/full.yaml",
            },
        },
    },
    "cilium": {
        "release_name": "cilium",
        "namespace": "kube-system",
        "create_namespace": False,
        "include_crds": True,
        "renders": {
            "bootstrap": {
                "tracked": True,
                "include_hooks": False,
                "strip_hook_annotations": False,
                "values": ("bootstrap-values.yaml",),
                "output": "render/bootstrap.yaml",
            },
            "full": {
                "tracked": False,
                "include_hooks": False,
                "strip_hook_annotations": False,
                "values": (),
                "output": ".state/render/full.yaml",
            },
        },
    },
    "kro": {
        "release_name": "kro",
        "namespace": "kro-system",
        "create_namespace": True,
        "include_crds": True,
        "renders": {
            "full": {
                "tracked": True,
                "include_hooks": False,
                "strip_hook_annotations": False,
                "values": (),
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
        "validate", help="Verify bootstrap charts lint clean and tracked renders are current"
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

        chart_path = directory / "Chart.yaml"
        version_path = directory / "VERSION"
        values_path = directory / "values.yaml"
        if not chart_path.is_file():
            raise BootstrapError(f"missing Chart.yaml: {chart_path}")
        if not version_path.is_file():
            raise BootstrapError(f"missing VERSION file: {version_path}")
        if not values_path.is_file():
            raise BootstrapError(f"missing values.yaml: {values_path}")

        chart = load_yaml(chart_path)
        wrapper_version = version_path.read_text(encoding="utf-8").strip()
        dependency = load_dependency(name, chart, chart_path)
        app_version = str(chart.get("appVersion", ""))

        if chart.get("name") != name:
            raise BootstrapError(f"{chart_path} must declare name: {name}")
        if str(chart.get("version", "")) != wrapper_version:
            raise BootstrapError(f"{chart_path} version must match {version_path}")
        if app_version != dependency.version:
            raise BootstrapError(f"{chart_path} appVersion must match dependency version")

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
            chart_path=chart_path,
            wrapper_version=wrapper_version,
            app_version=app_version,
            dependency=dependency,
            release_name=metadata["release_name"],
            namespace=metadata["namespace"],
            create_namespace=metadata["create_namespace"],
            include_crds=metadata["include_crds"],
            renders=renders,
        )

    return components


def load_yaml(path: Path) -> dict[str, Any]:
    try:
        document = yaml.safe_load(path.read_text(encoding="utf-8"))
    except yaml.YAMLError as exc:
        raise BootstrapError(f"failed to parse YAML: {path}: {exc}") from exc
    if not isinstance(document, dict):
        raise BootstrapError(f"expected a YAML mapping in {path}")
    return document


def load_dependency(component: str, chart: dict[str, Any], chart_path: Path) -> Dependency:
    dependencies = chart.get("dependencies")
    if not isinstance(dependencies, list) or len(dependencies) != 1:
        raise BootstrapError(f"{chart_path} must declare exactly one dependency")

    raw_dependency = dependencies[0]
    if not isinstance(raw_dependency, dict):
        raise BootstrapError(f"{chart_path} dependency must be a YAML mapping")

    name = str(raw_dependency.get("name", ""))
    alias = str(raw_dependency.get("alias", name))
    repository = str(raw_dependency.get("repository", ""))
    version = str(raw_dependency.get("version", ""))

    if not name or not repository or not version:
        raise BootstrapError(f"{chart_path} dependency must define name, repository, and version")

    return Dependency(name=name, alias=alias, repository=repository, version=version)


def select_components(components: dict[str, Component], name: str | None) -> list[Component]:
    if name is None:
        return [components[key] for key in sorted(components)]
    if name not in components:
        raise BootstrapError(f"unknown component: {name}")
    return [components[name]]


def list_components(components: list[Component]) -> None:
    for component in components:
        print(
            f"{component.name}\tbootstrap/{component.name}@{component.wrapper_version}\t"
            f"{component.namespace}\t"
            f"{component.dependency.repository}/{component.dependency.name}@{component.dependency.version}"
        )
        for lane in component.renders.values():
            tracked = "tracked" if lane.tracked else "local"
            print(f"  - {lane.name}: {tracked} -> {lane.output.relative_to(component.directory)}")


def render_components(components: list[Component], *, tracked_only: bool) -> None:
    for component in components:
        ensure_chart_dependencies(component)
        for lane in component.renders.values():
            if tracked_only and not lane.tracked:
                continue
            text = render_lane_text(component, lane)
            lane.output.parent.mkdir(parents=True, exist_ok=True)
            lane.output.write_text(text, encoding="utf-8")
            print(f"rendered {component.name}/{lane.name}: {lane.output.relative_to(ROOT)}")


def validate_components(components: list[Component]) -> None:
    for component in components:
        ensure_chart_dependencies(component)
        lint_component(component)
        for lane in component.renders.values():
            lint_component(component, lane)
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


def ensure_chart_dependencies(component: Component) -> None:
    run(["helm", "dependency", "build", str(component.directory)])


def lint_component(component: Component, lane: RenderLane | None = None) -> None:
    cmd = ["helm", "lint", str(component.directory)]
    for value_file in lane.values if lane else ():
        if not value_file.is_file():
            raise BootstrapError(f"missing values file: {value_file}")
        cmd.extend(["--values", str(value_file)])
    run(cmd)


def render_lane_text(component: Component, lane: RenderLane) -> str:
    cmd = [
        "helm",
        "template",
        component.release_name,
        str(component.directory),
        "--namespace",
        component.namespace,
        "--skip-tests",
    ]
    if not lane.include_hooks:
        cmd.append("--no-hooks")
    if component.include_crds:
        cmd.append("--include-crds")
    for value_file in lane.values:
        if not value_file.is_file():
            raise BootstrapError(f"missing values file: {value_file}")
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
        f"# Wrapper chart: bootstrap/{component.name}@{component.wrapper_version}",
        (
            f"# Upstream chart: {component.dependency.repository}/"
            f"{component.dependency.name}@{component.dependency.version}"
        ),
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
