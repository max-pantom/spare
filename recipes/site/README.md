# Site recipe

Site serves one selected folder read-only on localhost and the local network.
It serves `index.html` for directories, disables directory listings, denies
dotfiles and traversal, and permits symlinks only when they remain inside the
selected root.

See the [built-in recipes guide](../../docs/BUILT-IN-RECIPES.md#site) for setup,
persistent installation, backup, and safe-use instructions.

```bash
spare recipe validate ./recipes/site
spare recipe pack ./recipes/site
spare try site ./public
```
