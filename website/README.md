# Spare job catalog

This static site publishes the first-party optional-job catalog. Generate the
signed packages and `catalog.json` with:

```bash
SPARE_CATALOG_SIGNING_KEY=/secure/path/catalog-ed25519.pem \
  make catalog VERSION=0.1.1-alpha.3
```

The private signing key must remain outside the repository. Only jobs whose
trusted implementations ship in the corresponding Spare release may be
marked `available`.
