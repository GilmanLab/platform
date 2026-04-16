# GilmanLab Platform

This repository is for reusable platform components, shared manifests, and
supporting automation for the GilmanLab homelab.

The current baseline focuses on repo scaffolding, Moon-based CI, and GitHub
automation so platform projects can be added incrementally without replacing
the repository foundation each time.

## Quick Start

Prerequisites:

- `moon` 2.x

Validate the current repository baseline:

```sh
moon ci --summary minimal
```

Run the root check target directly:

```sh
moon run :check
```

## Support

- Questions and design discussion: GitHub Discussions
- Non-security bugs: GitHub Issues
- Vulnerabilities: follow [SECURITY.md](SECURITY.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
