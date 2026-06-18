# Self-hosted draw.io assets (deploy-only)

The wiki's diagram editor (`[[diagram:N]]`) embeds draw.io from a URL configured
in **Settings → Integrations → draw.io** (`drawio_url`), defaulting to
`/static/drawio/index.html` — i.e. this directory.

For a fully offline / LAN deployment, place the draw.io web app build here so the
default URL resolves. The assets are **not bundled in the repository** because of
their size; add them at image-build/deploy time, e.g.:

1. Download a draw.io (drawio-dev/drawio) release web build.
2. Copy its contents into `web/static/drawio/` so `index.html` sits next to this
   README.
3. (Optional) point `drawio_url` at an external draw.io instead.

The editor uses the embed protocol (`?embed=1&proto=json`): the app stores each
diagram's XML and a client-rendered PNG in the database (`wiki_diagrams`), and the
page only references it by id — keeping the article source clean.
