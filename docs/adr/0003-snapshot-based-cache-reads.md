# Snapshot-based cache reads

nogo copies `notion.db` (and its `-wal`/`-shm` files) to a temp file before
opening it. This avoids SQLite locking conflicts with the running Notion app
and ensures consistent reads even during an active sync.

The snapshot is cleaned up on `Reader.Close()`. There's no WAL replay — the
copy is a point-in-time snapshot, so partially-synced states are possible but
never corrupt.