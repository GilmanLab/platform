# labctl Architecture Guide

This document is a lightweight continuity guide for `labctl`. It is not a full
architecture spec and should not try to predict every future command. Extend it
only when real use cases teach us something worth standardizing.

The goal is simple: many small additions over time should still feel like one
coherent CLI instead of a pile of unrelated command handlers.

## File Structure

Use this structure as the default shape. Do not create empty directories before
they are needed, but keep new code moving toward these boundaries.

```text
tools/labctl/
  cmd/labctl/
    main.go                 # process entrypoint, signals, exit code
    main_test.go            # testscript registration once commands exist
    testdata/script/*.txtar # CLI integration tests

  internal/cli/
    root.go                 # root Cobra command construction
    config.go               # Viper setup and global flags
    errors.go               # CLI-facing error rendering and exit mapping
    output.go               # human/json output helpers
    progress.go             # progress helpers
    commands/
      <category>/
        command.go          # category command constructor
        <resource>.go       # resource or workflow commands

  internal/composition/
    composition.go          # manual wiring of app services and adapters

  internal/app/
    <capability>/
      service.go            # use cases and orchestration
      ports.go              # required external interfaces
      types.go              # request/result/domain value types

  internal/adapters/
    <system>/
      *.go                  # filesystem, process, network, API adapters

  internal/ui/
    logging/
      logging.go            # slog and Charmbracelet log wiring
    style/
      style.go              # shared Lip Gloss/Huh styling
```

`cmd/labctl` must stay thin. It owns process concerns only: context setup,
standard streams, version metadata, and converting the final error into an exit
code. Cobra command construction belongs under `internal/cli`. Business logic
belongs under `internal/app`. Anything that talks to the outside world belongs
under `internal/adapters`. The one package allowed to know about all concrete
pieces is `internal/composition`.

Avoid packages named `utils`, `helpers`, or `models`. If a package does not have
a crisp contract, keep refining the boundary before adding it.

## Dependency Injection

Use explicit manual dependency injection. Do not add a DI framework unless the
manual wiring becomes a proven, repeated maintenance problem.

The dependency graph should have one composition root:

```text
cmd/labctl/main.go
  -> internal/cli.Run(...)
  -> internal/composition.New(...)
  -> internal/app services + internal/adapters
```

`cmd/labctl` should pass process concerns into `internal/cli`: args, context,
standard streams, version metadata, and the process environment if needed. It
should not construct domain services or adapters directly.

`internal/composition` should build the concrete runtime:

```go
type Runtime struct {
	Logger   *slog.Logger
	Output   cli.Output
	Progress cli.Progress
	Prompter cli.Prompter
	Config   Config
}

type Dependencies struct {
	Runtime Runtime

	Bootstrap bootstrap.Dependencies
	Network   network.Dependencies
}
```

Keep this structure layered. A top-level `Dependencies` struct may group command
families, but command constructors should receive only what they need:

```go
root.AddCommand(bootstrapcmd.NewCommand(deps.Runtime, deps.Bootstrap))
root.AddCommand(networkcmd.NewCommand(deps.Runtime, deps.Network))
```

Avoid passing a giant dependency bag into every command. If a command needs a
long list of unrelated services, split the command family or introduce a focused
application service that owns the orchestration.

Application packages define their own ports and dependency structs:

```go
type Dependencies struct {
	Images ImageStore
	Runner CommandRunner
}

type ImageStore interface {
	Put(ctx context.Context, image Image) error
}
```

Adapters implement those ports. The app layer should not import
`internal/adapters`, `internal/cli`, Cobra, Viper, or Charmbracelet packages.

Do not store dependencies in `context.Context`. Use context for cancellation,
deadlines, and request-scoped values only. Do not use package-global service
registries, package-global Viper instances, or package-global Cobra commands.

## Command Definitions

Prefer constructor-based command registration. Do not use `init()` to attach
subcommands. Constructor wiring keeps dependencies explicit and makes tests
straightforward.

The root entrypoint should look like this shape:

```go
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	code := cli.Run(ctx, os.Args[1:], cli.IO{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	os.Exit(code)
}
```

The root CLI package should build a fresh command tree for each run:

```go
func NewRootCommand(deps Dependencies) *cobra.Command {
	opts := RootOptions{}

	cmd := &cobra.Command{
		Use:          "labctl",
		Short:        "Operate the GilmanLab homelab",
		SilenceUsage: true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return deps.Runtime.Initialize(cmd.Context(), cmd, opts)
		},
	}

	bindGlobalFlags(cmd, &opts)
	cmd.AddCommand(bootstrap.NewCommand(deps))

	return cmd
}
```

Each category or leaf command should expose a `NewCommand` constructor:

```go
func NewCommand(deps cli.Dependencies) *cobra.Command {
	opts := Options{}

	cmd := &cobra.Command{
		Use:   "example <target>",
		Short: "Do one clear operation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			request := app.Request{
				Target: app.Target(args[0]),
				DryRun: opts.DryRun,
			}

			result, err := deps.Example.Run(cmd.Context(), request)
			if err != nil {
				return err
			}

			return deps.Output.Write(cmd.Context(), result)
		},
	}

	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "preview without changing state")

	return cmd
}
```

Command handlers should translate CLI input into typed application requests,
call one use case, and render the result. They should not contain business
logic, filesystem logic, network clients, process execution, or long
orchestration flows.

## Command Hierarchy

`labctl` will span several operational subjects. Prefer command paths shaped as:

```text
labctl <category> <resource-or-workflow> <verb>
```

Examples of the shape, not committed categories:

```text
labctl bootstrap image build
labctl bootstrap cluster status
labctl network dns sync
labctl cluster node list
```

Use the same categorization process everywhere:

- Start with the operator's domain: `bootstrap`, `network`, `cluster`, `secrets`.
- Nest by resource or workflow when the category is broad.
- Use verbs consistently across categories: `list`, `show`, `apply`, `delete`,
  `sync`, `build`, `status`.
- Keep aliases rare. If a command needs aliases to feel usable, reconsider the
  command name first.

Do not design the full tree up front. Add hierarchy when working commands reveal
real boundaries.

## Global Flags And Configuration

Use Cobra for flags and command execution. Use an instance-scoped Viper for
configuration loading. Avoid package-global Viper state.

Global flags:

- `-v`, `--verbose`: repeatable count. `-v` enables info logs, `-vv` enables
  debug logs, and `-vvv` enables trace-level diagnostics where the logging
  adapter supports them.
- `-q`, `--quiet`: suppress human-facing output. Exit codes become the primary
  automation contract. Commands that produce data should still honor explicit
  machine output such as `--json` unless documented otherwise.
- `--non-interactive`: disable prompts, animated progress, styling that depends
  on a TTY, and any behavior that can hang automation. Reserve `-i` for this
  flag if a shorthand is added; do not reuse `-i` for command-local flags.
- `--config`: explicit config file path when config files become useful.

Configuration precedence should remain:

```text
flags > environment > config file > defaults
```

Bind flags to Viper only when they are configuration inputs. If a flag is just a
one-off execution option, read it from Cobra directly.

## Logging

Use `log/slog` as the core logging contract. Use Charmbracelet's log package at
the CLI edge to render human-friendly logs.

Logging rules:

- Default output should show warnings and errors only.
- `-v` enables informational progress and decisions.
- `-vv` enables debug details useful for troubleshooting.
- `-vvv` enables the most detailed adapter and request diagnostics that are safe
  to print.
- `--quiet` routes logs to discard unless a command explicitly writes a machine
  result.
- Logs and progress go to stderr. Command results go to stdout.

Do not create command-local log level mappings. Centralize logger construction
under `internal/ui/logging` or equivalent so every command behaves the same.

## Interactivity

Interactivity is an adapter concern, not business logic.

Use Charmbracelet libraries for interactive and styled surfaces:

- `huh` for confirmations, forms, and deliberate prompts.
- `lipgloss` for styled summaries, tables, and status blocks.
- Charmbracelet log styling for human diagnostics.

Interactive behavior must have a non-interactive path. If a command can prompt,
it must also accept flags or configuration that let automation make the same
choice. Destructive commands should provide `--dry-run` and an explicit
confirmation bypass such as `--yes` when needed.

Disable prompts, color-sensitive layout, and animated progress when:

- `--non-interactive` is set
- `--quiet` is set
- stdout/stderr is not a TTY and the output would be ambiguous
- `--json` is selected

## Results

Commands that produce structured data should support `--json` unless the output
is inherently human-only.

Result rules:

- Human output is the default.
- JSON output must be stable, plain, and free of styling.
- JSON goes to stdout.
- Diagnostics, logs, prompts, and progress go to stderr.
- Avoid mixing multiple unrelated result shapes in one command. If output needs
  separate modes, define typed result structs and renderers for each mode.

The application layer should return typed result values. Rendering belongs in
the CLI adapter.

## Errors

Errors should be human-readable, actionable, and useful in scripts.

Use domain-level sentinel errors or typed errors for categories that callers need
to branch on:

```go
var ErrNotFound = errors.New("not found")
```

Wrap errors with context at boundaries:

```go
return fmt.Errorf("load cluster inventory %q: %w", name, err)
```

The CLI error adapter should map known errors to clear messages and meaningful
exit codes. Avoid string matching. Use `errors.Is` and `errors.As`.

Good error messages should answer:

- what failed
- which input or resource was involved
- what the operator can try next, when there is an obvious next step

Do not leak noisy dependency errors directly when a short contextual message
would be clearer. Keep the original error wrapped for debug logging.

## Progress

Long operations should not appear hung.

Use progress indicators for operations with known steps or measurable duration:

- downloads
- image builds
- file copies
- polling loops
- multi-step bootstrap workflows

Progress belongs on stderr and must be disabled for `--quiet`, `--json`,
`--non-interactive`, and non-TTY contexts. When progress is disabled, commands
should still log enough at `-v` or above to troubleshoot where time is spent.

Prefer deterministic progress bars for known totals. Use spinners only for
unknown waits, and include a concise status label that names the current
operation.
