## Beskar

Beskar is an Artifact Registry based on [Docker Distribution Registry](https://github.com/distribution/distribution).
It's designed to support various artifacts and expose them through dedicated plugins.

### Features

* Modular/Extensible via [plugins](docs/plugins.md)
* Support for YUM repositories (beskar-yum)
* Support for static file repositories (beskar-static)
* Support for OSTree repositories ([beskar-ostree](internal/plugins/ostree/README.md))

### Docker images

Docker images are available for various architecture via Github packages repositories:

* [beskar](https://github.com/ctrliq/beskar/pkgs/container/beskar)
* [beskar-yum](https://github.com/ctrliq/beskar/pkgs/container/beskar-yum)
* [beskar-static](https://github.com/ctrliq/beskar/pkgs/container/beskar-static)
* [beskar-ostree](https://github.com/ctrliq/beskar/pkgs/container/beskar-ostree)

### Helm charts

Helm charts are available [here](https://github.com/ctrliq/beskar/tree/main/charts).

You can also pull charts directly for a specific release via Github packages by running:

For beskar helm chart:

```
helm pull oci://ghcr.io/ctrliq/helm-charts/beskar --version 0.0.1 --untar
```

For beskar-yum helm chart:

```
helm pull oci://ghcr.io/ctrliq/helm-charts/beskar-yum --version 0.0.1 --untar
```

For beskar-static helm chart:

```
helm pull oci://ghcr.io/ctrliq/helm-charts/beskar-static --version 0.0.1 --untar
```

For beskar-ostree helm chart:

```
helm pull oci://ghcr.io/ctrliq/helm-charts/beskar-ostree --version 0.0.1 --untar
```

### Compilation

Binaries are not provided as part of releases, you can compile it yourself by running:

```
./scripts/mage build:all
```

And retrieve binaries in `build/output` directory.

**NOTE**: Require the Golang toolchain installation
