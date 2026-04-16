# Contributing

Thank you for contributing to `GilmanLab/platform`.

Use GitHub Discussions for design questions and broader platform planning. Use
GitHub Issues for non-security bugs. For vulnerabilities, stop and follow
[SECURITY.md](SECURITY.md) instead of using public channels.

## Pull Requests

1. Keep changes scoped to one platform component or support concern.
2. Update tests, validation, or docs when behavior changes.
3. Describe user-facing and operator-facing impact clearly in the pull request.
4. Make sure CI passes before requesting review.

## Local Setup

Validate the repository baseline:

```sh
moon ci --summary minimal
moon run :check
```
