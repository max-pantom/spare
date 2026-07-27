# Export and restore recipe data

Spare can export one installed instance's configuration, version, runtime,
port preference, metadata, and selected-folder files.

Only recipes with selected-folder data can be exported. Hook keeps temporary
history in memory and therefore has no backup source.

## Export

Export the installed recipe:

```bash
spare export drop
```

Choose an output path:

```bash
spare export drop --output ~/Backups/drop.spare-backup
```

The command reads the complete selected folder. It rejects symlinks and special
files rather than following data outside the selected root.

The archive contains:

```text
drop.spare-backup
├── backup.json
└── data/
    └── selected-folder files
```

Backups are ZIP-compatible and are not encrypted. Store them with the same care
as the original data.

## Restore

Choose a new or empty destination:

```bash
spare import ~/Backups/drop.spare-backup --path ~/Received-restored
```

Restore with automatic port selection instead of the saved preference:

```bash
spare import ~/Backups/drop.spare-backup \
  --path ~/Received-restored \
  --port auto
```

Restore:

1. Validates `backup.json`.
2. Requires an empty destination.
3. Rejects traversal and symlinks.
4. Extracts files without overwriting existing data.
5. Replaces the saved folder field with the new destination.
6. Installs the built-in recipe through the normal API and supervisor.

If data extraction succeeds but recipe installation fails, Spare reports the
restored destination. It does not delete restored files during recovery.
