# SQLite Writable Lifecycle

`sqlite.Open` is the writable application-store lifecycle. `sqlite.Load` remains
the separate read-only formatter snapshot path until startup composition adopts
the writable store.

- Empty files are initialized with the Java-compatible base tables and the
  append-only `jproxy_go_migration` ledger.
- An existing database must match the final Java changelog contract, including
  required columns and indexes. Unknown schema or ledger checksum drift fails
  closed before a migration is applied.
- Before the first Go migration of a compatible Java database, SQLite performs
  `VACUUM INTO <db>.bak-<UTC timestamp>` without checkpointing or otherwise
  mutating the source first. The backup must
  open, pass `integrity_check`, match required table row counts, and match the
  Java schema contract before Go writes the source database.
- The writer uses `foreign_keys=1`, a 5-second busy timeout, one connection,
  WAL mode, and `synchronous=FULL`. WAL makes readers concurrent with a writer;
  it does not permit concurrent writers.
- `<db>.jproxy-writer.lock` is an OS advisory lock held until `Store.Close`.
  It rejects another cooperating Go writer and is released on process exit. The
  Java application does not participate in this lock, so operational cutover
  must stop Java before Go obtains the writable lifecycle. Only local supported
  filesystems are covered; SMB, NFS, and other network filesystems are unsupported.

Backups, lock files, and real databases are runtime data and must not be
committed. Restore only into a stopped, separate copy before replacing a live
database.
