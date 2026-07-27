# Hook recipe

Hook is a local webhook inbox. It accepts any HTTP method at `/hook` and
`/hook/*`, keeps the latest 50 requests in memory, shows their method, path,
query, headers, source, and body, and can replay a captured request to a full
HTTP or HTTPS URL.

Request bodies are limited to 1 MB, and each request keeps its latest 20 replay
attempts. History lasts only while Hook is running; stopping or restarting the
recipe clears it. Replay preserves end-to-end headers and does not follow
redirects.

```bash
spare recipe validate ./recipes/hook
spare recipe pack ./recipes/hook
spare try hook
```

Hook has no accounts or TLS in this preview. Anyone who can reach its local
network URL can send and inspect requests, including secret header and body
values. Use it only on a network you trust.

See the [built-in recipes guide](../../docs/BUILT-IN-RECIPES.md#hook) for test
requests, replay, persistent installation, and safe-use instructions.
