#!/usr/bin/env python3
"""Validate the immutable image contract used by CI and release tooling."""

from __future__ import annotations

import argparse
import json
import re
import sys
from typing import Any


COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")
DIGEST_RE = re.compile(r"^sha256:[0-9a-f]{64}$")


class ContractError(ValueError):
    """Raised when a release input is mutable or does not match its build."""


def normalize_commit(value: str) -> str:
    commit = value.strip().lower()
    if not COMMIT_RE.fullmatch(commit):
        raise ContractError("commit must be a full 40-character hexadecimal SHA")
    return commit


def normalize_digest(value: str) -> str:
    digest = value.strip().lower()
    if not DIGEST_RE.fullmatch(digest):
        raise ContractError("digest must use sha256 followed by 64 hexadecimal characters")
    return digest


def split_image_tag(image_tag: str) -> tuple[str, str]:
    value = image_tag.strip()
    if not value or any(character.isspace() for character in value):
        raise ContractError("image tag must be non-empty and contain no whitespace")
    if "@" in value:
        raise ContractError("publish input must be a commit tag, not a digest reference")

    final_slash = value.rfind("/")
    tag_separator = value.rfind(":")
    if tag_separator <= final_slash:
        raise ContractError("publish input must include an explicit image tag")

    repository = value[:tag_separator]
    tag = value[tag_separator + 1 :]
    if not repository or not tag or "://" in repository:
        raise ContractError("publish input is not a valid registry image tag")
    return repository, tag


def validate_publish_tag(image_tag: str, expected_commit: str) -> str:
    commit = normalize_commit(expected_commit)
    repository, tag = split_image_tag(image_tag)
    if tag != commit:
        raise ContractError(
            "publish tag must equal the exact commit SHA; moving tags such as main are forbidden"
        )
    return repository


def validate_digest_ref(image_ref: str) -> str:
    value = image_ref.strip()
    if value.count("@") != 1 or any(character.isspace() for character in value):
        raise ContractError("release and rollback inputs must be an image digest reference")

    repository, digest = value.split("@", 1)
    if not repository or "://" in repository:
        raise ContractError("digest reference has an invalid image repository")
    if ":" in repository.rsplit("/", 1)[-1]:
        raise ContractError("digest reference must not include a tag")

    return f"{repository}@{normalize_digest(digest)}"


def canonicalize_deployed_ref(image_ref: str) -> str:
    """Convert Docker's optional ``repository:tag@digest`` form to digest-only."""

    value = image_ref.strip()
    if value.count("@") != 1 or any(character.isspace() for character in value):
        raise ContractError("deployed image must contain exactly one registry digest")

    repository, digest = value.split("@", 1)
    final_slash = repository.rfind("/")
    tag_separator = repository.rfind(":")
    if tag_separator > final_slash:
        repository = repository[:tag_separator]
    return validate_digest_ref(f"{repository}@{digest}")


def _load_single_inspect_object(payload: str) -> dict[str, Any]:
    try:
        decoded = json.loads(payload)
    except json.JSONDecodeError as exc:
        raise ContractError(f"docker image inspect returned invalid JSON: {exc.msg}") from exc

    if not isinstance(decoded, list) or len(decoded) != 1 or not isinstance(decoded[0], dict):
        raise ContractError("docker image inspect must return exactly one image object")
    return decoded[0]


def resolve_inspect_payload(
    payload: str,
    image_tag: str,
    expected_commit: str,
    expected_digest: str,
) -> str:
    repository = validate_publish_tag(image_tag, expected_commit)
    commit = normalize_commit(expected_commit)
    build_digest = normalize_digest(expected_digest)
    image = _load_single_inspect_object(payload)

    config = image.get("Config")
    labels = config.get("Labels") if isinstance(config, dict) else None
    revision = labels.get("org.opencontainers.image.revision") if isinstance(labels, dict) else None
    if revision != commit:
        raise ContractError(
            f"OCI revision mismatch: expected {commit}, got {revision or '<missing>'}"
        )

    repo_digests = image.get("RepoDigests")
    if not isinstance(repo_digests, list):
        raise ContractError("docker image inspect did not return RepoDigests")

    matching_refs: list[str] = []
    for candidate in repo_digests:
        if not isinstance(candidate, str) or "@" not in candidate:
            continue
        candidate_repository, candidate_digest = candidate.rsplit("@", 1)
        if candidate_repository != repository:
            continue
        try:
            normalized_ref = validate_digest_ref(candidate)
        except ContractError:
            continue
        if normalize_digest(candidate_digest) == build_digest:
            matching_refs.append(normalized_ref)

    if len(set(matching_refs)) != 1:
        raise ContractError(
            "registry digest does not match the build digest for the exact commit tag"
        )
    return matching_refs[0]


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    validate_publish = subparsers.add_parser(
        "validate-publish", help="validate an exact commit image tag"
    )
    validate_publish.add_argument("--image-tag", required=True)
    validate_publish.add_argument("--expected-commit", required=True)

    resolve_inspect = subparsers.add_parser(
        "resolve-inspect", help="validate docker inspect JSON and emit image@digest"
    )
    resolve_inspect.add_argument("--image-tag", required=True)
    resolve_inspect.add_argument("--expected-commit", required=True)
    resolve_inspect.add_argument("--expected-digest", required=True)

    validate_deploy = subparsers.add_parser(
        "validate-deploy-ref", help="reject tag-based release or rollback inputs"
    )
    validate_deploy.add_argument("image_ref")

    canonicalize_deployed = subparsers.add_parser(
        "canonicalize-deployed-ref",
        help="normalize Docker's tag@digest service value to digest-only",
    )
    canonicalize_deployed.add_argument("image_ref")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        if args.command == "validate-publish":
            validate_publish_tag(args.image_tag, args.expected_commit)
            return 0
        if args.command == "resolve-inspect":
            print(
                resolve_inspect_payload(
                    sys.stdin.read(),
                    args.image_tag,
                    args.expected_commit,
                    args.expected_digest,
                )
            )
            return 0
        if args.command == "validate-deploy-ref":
            print(validate_digest_ref(args.image_ref))
            return 0
        if args.command == "canonicalize-deployed-ref":
            print(canonicalize_deployed_ref(args.image_ref))
            return 0
    except ContractError as exc:
        print(f"release contract violation: {exc}", file=sys.stderr)
        return 2

    raise AssertionError(f"unhandled command: {args.command}")


if __name__ == "__main__":
    raise SystemExit(main())
