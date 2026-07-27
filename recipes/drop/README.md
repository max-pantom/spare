# Drop recipe

Drop receives one browser upload at a time into a selected folder. It provides
download links, upload progress, available-storage reporting, collision-safe
filenames, and an adjustable maximum file size.

Drop is local-network software without accounts, TLS, cloud storage, sync, or
remote access in this preview. Use it only on a network you trust.

See the [built-in recipes guide](../../docs/BUILT-IN-RECIPES.md#drop) for setup,
persistent installation, browser use, backup, and safe-use instructions.

```bash
spare recipe validate ./recipes/drop
spare recipe pack ./recipes/drop
spare try drop ./received-files
```
