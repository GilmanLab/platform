# Agent Instructions — labctl

This file applies to `tools/labctl/` and all descendants. Follow the workspace
and repository rules too; this file adds the local rules for this CLI.

## Required Skill Loading

Before starting substantive work in this subtree, load these skills:

- `charmbracelet` — Charmbracelet CLI UI libraries
- `cli` — command-line interface design and ergonomics
- `cobra-viper-cli` — Cobra/Viper CLI implementation
- `git` — version control operations
- `go-style` — Go package structure, APIs, comments, and observability style
- `go-testing` — Go unit-test design and behavior-focused test structure
- `go-testscript` — Go CLI integration tests with testscript/txtar
- `moonrepo` — Moon workspace, toolchain, task, and CI configuration
- `worktrunk` — worktree-based branch isolation

Do not treat these as optional references. Load them before editing code, tests,
Moon config, or command structure.

## Architecture

- Use hexagonal architecture and clean code principles. Keep business logic
  separate from CLI wiring, terminal UI, filesystem, network, storage, process
  execution, and other external adapters.
- Model external systems as ports at the boundary of the core behavior. Implement
  concrete adapters outside the core and inject them explicitly.
- Keep packages small and focused, with strong contractual boundaries. Split on
  behavior and contracts, not on vague buckets like `helpers`, `utils`, or
  `models`.
- Prefer Go's type system to enforce contracts and prevent bugs. Use typed
  configuration, domain-specific value types, narrow interfaces, explicit result
  structs, and compile-time validation where they make invalid states harder to
  express.
- Follow `go-style` guidance for package layout, exported API documentation,
  sparse comments, optional observability, and `slog.Logger`-compatible logging.

## CLI Design

- The CLI must be implemented with Cobra and Viper.
- Use Cobra for command structure, argument validation, help/completion surfaces,
  context propagation, and command execution. Prefer `RunE`, `ExecuteContext`,
  and explicit errors.
- Use Viper for configuration loading and precedence. Prefer instance-based
  Viper wiring over package-global configuration state.
- Keep Cobra/Viper code in the adapter layer. Command handlers should translate
  CLI input into typed application calls rather than holding business logic.
- Use Charmbracelet libraries for a streamlined terminal experience:
  `lipgloss` for styled summaries/layout, `huh` for deliberate interactive
  prompts, and `log` or `slog` integration for human-readable diagnostics.
- Preserve a scriptable core. Put command data on stdout; put prompts, progress,
  logs, and diagnostics on stderr. Interactive flows must not be the only path
  for automation.

## Command Hierarchy

`labctl` will span several operational subjects, so command hierarchy matters.

- Prefer a strong hierarchical command tree over a flat list of unrelated verbs.
- Use consistent categorization across boundaries. If a concept appears in more
  than one area, reuse the same noun and nesting pattern instead of inventing a
  second vocabulary.
- Group commands by the operator's mental model first, then by implementation
  detail. Command paths should make it obvious what resource or workflow they
  operate on.
- Keep verbs predictable inside each category. Avoid sibling commands that do
  nearly the same thing with different names.
- Do not over-design the full tree before behavior exists. Add hierarchy as
  working commands reveal real boundaries.

## Testing

- Code is not finished until behavior is tested. Prefer behavior-focused tests
  over coverage-driven tests.
- Unit tests should prove observable behavior through package boundaries. Use
  table-driven tests, helpers, Testify `assert`/`require`, and Mockery-generated
  mocks where collaborators need to be controlled.
- CLI behavior should have integration coverage with `go-testscript` and
  `.txtar` scripts once an executable command surface exists.
- Tests must not depend on live homelab services by default. Use fakes, mocks,
  `httptest`, temporary directories, or containerized dependencies before
  reaching for live systems.

## Completion Criteria

- Run the narrow task checks while iterating, but a change is not complete until
  `moon ci` passes from the platform repository root.
- Keep `tools/labctl/moon.yml` aligned with the actual development contract for
  this module. If new generated artifacts, test fixtures, or tool config files
  become part of the CLI workflow, add them to Moon inputs.
