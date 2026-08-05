# Version and release tags

Spare uses semantic versions with a leading `v` in Git tags:

```text
vMAJOR.MINOR.PATCH-STAGE.NUMBER
```

The human-readable GitHub release title may add the product name and a short
description. For example:

```text
Tag:   v0.1.1-alpha.1
Title: Spare 0.1.1 Alpha 1 — Desktop Preview
```

Every release before the first stable release must be marked as a
**pre-release** on GitHub.

## Current release line

The current desktop preview is:

```text
v0.1.1-alpha.3
```

Do not reuse or move a published tag. If another early desktop build is
needed, increment the final number:

```text
v0.1.1-alpha.2
v0.1.1-alpha.3
```

## When to use each stage

| Stage | Example | Use it when |
| --- | --- | --- |
| Development | No tag | Work is incomplete and intended only for local development. |
| Alpha | `v0.1.1-alpha.1` | The main feature exists, but platform testing, polish, or important fixes are still outstanding. Breakage is expected. |
| Beta | `v0.1.1-beta.1` | The planned feature set is present and suitable for broader testing, but bugs and compatibility problems may remain. |
| Release candidate | `v0.1.1-rc.1` | The build could become the stable release if final acceptance testing finds no blocking problems. |
| Stable | `v0.1.1` | Required acceptance testing has passed and the release is suitable for normal use within its documented limits. |

The number after a stage starts at `1` and increases for each build at that
stage:

```text
v0.1.1-alpha.1
v0.1.1-alpha.2
v0.1.1-beta.1
v0.1.1-beta.2
v0.1.1-rc.1
v0.1.1
```

Moving to a new stage resets that stage's build number to `1`.

## Choosing major, minor, and patch numbers

- Increment **PATCH** for fixes or a small preview iteration on the current
  release line: `0.1.0` to `0.1.1`.
- Increment **MINOR** for a meaningful new set of capabilities:
  `0.1.x` to `0.2.0`.
- Keep **MAJOR** at `0` while Spare's public interfaces and installation
  model may still change substantially.
- Use `1.0.0` only after the supported platforms, installation flow, security
  boundary, compatibility promises, and upgrade path are ready to be treated
  as stable.

## Current path to stable

Use this sequence unless the scope changes:

```text
v0.1.1-alpha.1   Spare 0.1.1 Alpha 1 — Desktop Preview
v0.1.1-alpha.2   Earlier desktop fixes
v0.1.1-alpha.3   Optional jobs and catalog preview
v0.1.1-beta.1    Broader desktop testing begins
v0.1.1-rc.1      Final acceptance candidate
v0.1.1           Stable 0.1.1 release
```

A stage may have more than one numbered build. Do not advance merely because
time has passed:

- Advance from alpha to beta when the intended feature set works and the
  known remaining work is primarily testing, compatibility, and polish.
- Advance from beta to release candidate when there are no known
  release-blocking defects.
- Remove the suffix only after the release candidate passes the required
  native platform and installer acceptance tests.

## Release checklist

Before creating a GitHub release:

1. Choose the next version using the rules above.
2. Update the version in the Makefile and the built-in recipe manifests.
3. Build and run the relevant automated tests.
4. Build the packages using the same version as the tag.
5. Verify every archive's architecture and checksum.
6. Push the exact release commit.
7. Create a new GitHub tag targeting that commit.
8. Give the release a human-readable title.
9. Select **Set as a pre-release** for alpha, beta, and release-candidate
   builds.
10. Publish the release and confirm that its generated assets use the expected
    version.
11. Test the downloaded artifacts rather than relying only on local build
    output.

The release tag is the version passed to the GitHub packaging workflow.
Publishing `v0.1.1-alpha.1`, for example, produces artifacts named with
`0.1.1-alpha.1`. A normal push to `main` does not create or increment a
release.

## Release notes

Release-specific notes are kept in [`releases/`](releases/). The first desktop
release notes are:

- [Spare 0.1.1 Alpha 1 — Desktop Preview](releases/0.1.1-alpha.1.md)
