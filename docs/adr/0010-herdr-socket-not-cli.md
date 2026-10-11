# blf talks to the herdr socket, not the herdr CLI

`blf herdr sync-tab-title` runs on every fish prompt and command, so each call to herdr counts. Spawning `herdr` costs about 4 ms per call (almost all of it process start), while the same request sent straight to `$HERDR_SOCKET_PATH` costs 0.05–0.2 ms. A sync needs two or three calls, so blf speaks the socket protocol directly from Go instead of shelling out.

## Consequences

- The socket protocol is less stable than the CLI: a herdr update can change a reply and break the sync. blf was written against herdr 0.9.3 (protocol 22).
- To keep that failure visible, replies are decoded strictly (a missing field is an error that names the tested herdr version), and `blf herdr doctor` runs read-only checks against the live session.
- A newer herdr version is not treated as a fault by itself, so there is no version warning on each sync.
