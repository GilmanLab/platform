# List the available platform bootstrap recipes.
default:
    @just --list

# Render the tracked bootstrap artifacts into their committed paths.
render:
    uv run scripts/render_bootstrap.py render --tracked-only

# Render all bootstrap artifacts, including local-only private outputs.
render-all:
    uv run scripts/render_bootstrap.py render

# Render the tracked RGD bundle artifacts into their committed paths.
render-rgds:
    uv run scripts/render_rgds.py render

# Lint bootstrap charts and verify tracked bootstrap artifacts are in sync and free of embedded secret material.
validate:
    uv run scripts/render_bootstrap.py validate

# Verify tracked RGD bundle artifacts are in sync and internally consistent.
validate-rgds:
    uv run scripts/render_rgds.py validate

# Render tracked artifacts and verify they are current.
check: render render-rgds validate validate-rgds

# Show the configured bootstrap components and render lanes.
list:
    uv run scripts/render_bootstrap.py list

# Show the configured RGD bundles and tracked renders.
list-rgds:
    uv run scripts/render_rgds.py list
