# Self-hosted draw.io assets

The wiki diagram editor (`[[diagram:N]]`) embeds draw.io from this directory
(`/static/drawio/`), configurable in **Settings → Integrations → draw.io**
(`drawio_url`).

**The container build downloads draw.io automatically** — see `Containerfile`
(the `assets` stage fetches `draw.war` from the jgraph/drawio release and
unzips it here). So a normal `podman-compose build` produces a working,
offline, self-hosted editor; no manual step is needed.

To change the version or skip the download, edit the draw.io step in
`Containerfile`. For a non-container run, drop a draw.io web build into this
folder so `index.html` sits next to this README, or point `drawio_url` at an
external draw.io.

Diagrams are stored in the database (`wiki_diagrams`: XML + a client-rendered
PNG); pages only reference them by id, keeping the article source clean.
