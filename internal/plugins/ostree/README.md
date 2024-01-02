# OSTree Plugin

## Overview
The ostree plugin is responsible for mapping the ostree repository to the OCI registry. This is done in the router.rego
and no routing execution happens within the plugin itself at runtime. The plugin does, however, provide an API for mirroring
ostree repositories into beskar.

## File Tagging
The ostree plugin maps the ostree repository filepaths to the OCI registry tags. Most files are simply mapped by hashing
the full filepath relative to the ostree root. For example, `objects/ab/abcd1234.filez` becomes `file:b8458bd029a97ca5e03f272a6b7bd0d1`.
There are a few exceptions to this rule, however. The following files are considered "special" and are tagged as follows:
1. `summary` -> `file:summary`
2. `summary.sig` -> `file:summary.sig`
3. `config` -> `file:config`

There is no technical reason for this and is only done to make the mapping more human-readable in the case of "special"
files.

## Mirroring
TBD