# List the available platform bootstrap recipes.
default:
    @just --list

# Render the tracked bootstrap artifacts into their committed paths.
render:
    uv run scripts/render_bootstrap.py render --tracked-only

# Render all bootstrap artifacts, including local-only private outputs.
render-all:
    uv run scripts/render_bootstrap.py render

# Verify tracked bootstrap artifacts are in sync and free of embedded secret material.
validate:
    uv run scripts/render_bootstrap.py validate

# Render tracked artifacts and verify they are current.
check: render validate

# Show the configured bootstrap components and render lanes.
list:
    uv run scripts/render_bootstrap.py list
