#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "PyYAML>=6.0,<7",
# ]
# ///
"""Render and validate tracked RGD bundles from repo conventions."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
from pathlib import Path
import re
import shutil
import subprocess
import sys
from typing import Any

import yaml


ROOT = Path(__file__).resolve().parents[1]
REQUIRED_TOOLS = ("cue",)
SEMVER_RE = re.compile(r"^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$")


class RgdError(RuntimeError):
    """Raised when RGD inputs or rendering steps are invalid."""


@dataclass(frozen=True)
class BundleMetadata:
    name: str
    package: str
    artifact: str
    rgd_name: str
    api_group: str
    api_version: str
    api_kind: str
    api_scope: str


@dataclass(frozen=True)
class Bundle:
    key: str
    directory: Path
    version_path: Path
    changelog_path: Path
    render_output: Path
    metadata_input: Path
    metadata_expression: str
    output_expression: str
    version: str
    metadata: BundleMetadata


BUNDLE_METADATA: dict[str, dict[str, str]] = {
    "platform": {
        "directory": "rgds/platform",
        "metadata_input": "bundle.cue",
        "render_output": "render/platform-rgds.yaml",
        "metadata_expression": "bundle",
        "output_expression": "output",
    },
}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    list_parser = subparsers.add_parser("list", help="Show configured RGD bundles")
    list_parser.add_argument("--bundle", help="Restrict output to one bundle")

    render_parser = subparsers.add_parser("render", help="Render tracked RGD bundles")
    render_parser.add_argument("--bundle", help="Restrict output to one bundle")

    validate_parser = subparsers.add_parser(
        "validate", help="Verify tracked RGD bundles are current and internally consistent"
    )
    validate_parser.add_argument("--bundle", help="Restrict validation to one bundle")

    args = parser.parse_args()

    try:
        ensure_tools()
        bundles = load_bundles()
        selected = select_bundles(bundles, args.bundle)

        if args.command == "list":
            list_bundles(selected)
        elif args.command == "render":
            render_bundles(selected)
        elif args.command == "validate":
            validate_bundles(selected)
        else:
            raise RgdError(f"unsupported command: {args.command}")
    except RgdError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    return 0


def ensure_tools() -> None:
    missing = [tool for tool in REQUIRED_TOOLS if shutil.which(tool) is None]
    if missing:
        raise RgdError(f"missing required tool(s): {', '.join(missing)}")


def run(cmd: list[str], *, cwd: Path | None = None) -> str:
    try:
        result = subprocess.run(cmd, check=True, text=True, capture_output=True, cwd=cwd)
    except subprocess.CalledProcessError as exc:
        message = exc.stderr.strip() or exc.stdout.strip() or "command failed"
        raise RgdError(f"{' '.join(cmd)}: {message}") from exc

    return result.stdout


def load_bundles() -> dict[str, Bundle]:
    bundles: dict[str, Bundle] = {}
    for key, config in sorted(BUNDLE_METADATA.items()):
        directory = ROOT / config["directory"]
        version_path = directory / "VERSION"
        changelog_path = directory / "CHANGELOG.md"
        metadata_input = directory / config["metadata_input"]
        render_output = directory / config["render_output"]

        if not directory.is_dir():
            raise RgdError(f"bundle directory does not exist: {directory}")
        if not version_path.is_file():
            raise RgdError(f"missing VERSION file: {version_path}")
        if not changelog_path.is_file():
            raise RgdError(f"missing CHANGELOG.md: {changelog_path}")

        version = version_path.read_text(encoding="utf-8").strip()
        if not SEMVER_RE.match(version):
            raise RgdError(f"{version_path} must contain a semantic version")

        changelog_header = changelog_path.read_text(encoding="utf-8").splitlines()
        if not changelog_header or changelog_header[0] != "# Changelog":
            raise RgdError(f"{changelog_path} must begin with '# Changelog'")

        metadata = load_bundle_metadata(metadata_input, config["metadata_expression"])

        bundles[key] = Bundle(
            key=key,
            directory=directory,
            version_path=version_path,
            changelog_path=changelog_path,
            render_output=render_output,
            metadata_input=metadata_input,
            metadata_expression=config["metadata_expression"],
            output_expression=config["output_expression"],
            version=version,
            metadata=metadata,
        )

    return bundles


def load_bundle_metadata(metadata_input: Path, expression: str) -> BundleMetadata:
    raw = run(
        [
            "cue",
            "export",
            f"./{metadata_input.relative_to(ROOT)}",
            "--out",
            "yaml",
            "--expression",
            expression,
        ],
        cwd=ROOT,
    )
    document = load_yaml_text(raw)

    try:
        name = str(document["name"])
        package = str(document["package"])
        artifact = str(document["artifact"])
        rgd_name = str(document["rgdName"])
        api = document["api"]
        if not isinstance(api, dict):
            raise TypeError("bundle.api must be a mapping")
        api_group = str(api["group"])
        api_version = str(api["version"])
        api_kind = str(api["kind"])
        api_scope = str(api["scope"])
    except (KeyError, TypeError) as exc:
        raise RgdError(f"bundle metadata in {metadata_input} is incomplete: {exc}") from exc

    return BundleMetadata(
        name=name,
        package=package,
        artifact=artifact,
        rgd_name=rgd_name,
        api_group=api_group,
        api_version=api_version,
        api_kind=api_kind,
        api_scope=api_scope,
    )


def load_yaml_text(text: str) -> dict[str, Any]:
    try:
        document = yaml.safe_load(text)
    except yaml.YAMLError as exc:
        raise RgdError(f"failed to parse YAML output: {exc}") from exc
    if not isinstance(document, dict):
        raise RgdError("expected YAML mapping output")
    return document


def select_bundles(bundles: dict[str, Bundle], key: str | None) -> list[Bundle]:
    if key is None:
        return [bundles[name] for name in sorted(bundles)]
    if key not in bundles:
        raise RgdError(f"unknown bundle: {key}")
    return [bundles[key]]


def list_bundles(bundles: list[Bundle]) -> None:
    for bundle in bundles:
        metadata = bundle.metadata
        print(
            f"{bundle.key}\t{metadata.package}@{bundle.version}\t"
            f"{metadata.api_kind}.{metadata.api_group}/{metadata.api_version}\t"
            f"{metadata.artifact}"
        )
        print(f"  - render: tracked -> {bundle.render_output.relative_to(bundle.directory)}")


def render_bundles(bundles: list[Bundle]) -> None:
    for bundle in bundles:
        text = render_bundle_text(bundle)
        bundle.render_output.parent.mkdir(parents=True, exist_ok=True)
        bundle.render_output.write_text(text, encoding="utf-8")
        print(f"rendered {bundle.key}: {bundle.render_output.relative_to(ROOT)}")


def validate_bundles(bundles: list[Bundle]) -> None:
    for bundle in bundles:
        expected = render_bundle_text(bundle)
        if not bundle.render_output.is_file():
            raise RgdError(f"tracked render is missing: {bundle.render_output}")

        current = bundle.render_output.read_text(encoding="utf-8")
        if current != expected:
            raise RgdError(
                f"tracked render is out of date: {bundle.render_output.relative_to(ROOT)}; "
                "run `just render-rgds`"
            )

        documents = [doc for doc in yaml.safe_load_all(current) if doc is not None]
        if len(documents) != 1:
            raise RgdError(
                f"tracked render must contain exactly one document: "
                f"{bundle.render_output.relative_to(ROOT)}"
            )

        validate_rendered_document(bundle, documents[0])
        print(f"validated {bundle.key}: {bundle.render_output.relative_to(ROOT)}")


def render_bundle_text(bundle: Bundle) -> str:
    raw = run(
        [
            "cue",
            "export",
            f"./{bundle.directory.relative_to(ROOT)}",
            "--out",
            "yaml",
            "--expression",
            bundle.output_expression,
        ],
        cwd=ROOT,
    )

    try:
        documents = [doc for doc in yaml.safe_load_all(raw) if doc is not None]
    except yaml.YAMLError as exc:
        raise RgdError(f"failed to parse rendered YAML for {bundle.key}: {exc}") from exc

    if not documents:
        raise RgdError(f"rendered output is empty for {bundle.key}")

    header = [
        "# Generated by scripts/render_rgds.py; do not edit by hand.",
        f"# Bundle: {bundle.metadata.name}",
        f"# Package: {bundle.metadata.package}",
        f"# Artifact: {bundle.metadata.artifact}@{bundle.version}",
        (
            f"# API: {bundle.metadata.api_kind}."
            f"{bundle.metadata.api_group}/{bundle.metadata.api_version}"
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


def validate_rendered_document(bundle: Bundle, document: dict[str, Any]) -> None:
    metadata = bundle.metadata

    if document.get("apiVersion") != "kro.run/v1alpha1":
        raise RgdError(f"{bundle.key} render must use apiVersion kro.run/v1alpha1")
    if document.get("kind") != "ResourceGraphDefinition":
        raise RgdError(f"{bundle.key} render must use kind ResourceGraphDefinition")

    rendered_metadata = document.get("metadata")
    if not isinstance(rendered_metadata, dict):
        raise RgdError(f"{bundle.key} render must include metadata")
    if rendered_metadata.get("name") != metadata.rgd_name:
        raise RgdError(f"{bundle.key} render must set metadata.name={metadata.rgd_name}")

    spec = document.get("spec")
    if not isinstance(spec, dict):
        raise RgdError(f"{bundle.key} render must include spec")

    schema = spec.get("schema")
    if not isinstance(schema, dict):
        raise RgdError(f"{bundle.key} render must include spec.schema")

    if schema.get("apiVersion") != metadata.api_version:
        raise RgdError(f"{bundle.key} render must set spec.schema.apiVersion={metadata.api_version}")
    if schema.get("group") != metadata.api_group:
        raise RgdError(f"{bundle.key} render must set spec.schema.group={metadata.api_group}")
    if schema.get("kind") != metadata.api_kind:
        raise RgdError(f"{bundle.key} render must set spec.schema.kind={metadata.api_kind}")
    if schema.get("scope") != metadata.api_scope:
        raise RgdError(f"{bundle.key} render must set spec.schema.scope={metadata.api_scope}")

    schema_spec = schema.get("spec")
    if not isinstance(schema_spec, dict):
        raise RgdError(f"{bundle.key} render must include spec.schema.spec")

    dns = schema_spec.get("dns")
    tls = schema_spec.get("tls")
    if not isinstance(dns, dict) or "zone" not in dns:
        raise RgdError(f"{bundle.key} render must include spec.schema.spec.dns.zone")
    if not isinstance(tls, dict) or "clusterIssuer" not in tls:
        raise RgdError(f"{bundle.key} render must include spec.schema.spec.tls.clusterIssuer")

    resources = spec.get("resources")
    if not isinstance(resources, list):
        raise RgdError(f"{bundle.key} render must include spec.resources")


if __name__ == "__main__":
    raise SystemExit(main())
