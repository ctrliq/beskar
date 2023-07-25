### RPM packages in OCI registry without DB

All references below would be handled by plugins.

This reference holds an index manifest pointing to various metadata manifests (and for a revision):

```
/<project|org_name>/yum/<repo_name>:latest
/<project|org_name>/yum/<repo_name>:<revision>
```

This reference holds an index manifest pointing to signing keys manifests:

```
/<project|org_name>/yum/<repo_name>/signing-keys:latest
```

This reference holds an index manifest pointing to all repository package manifests (could take few MB for big repo):

```
/<project|org_name>/yum/<repo_name>/packages:latest
```

Package index manifest format:
```
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.index.v1+json",
  "manifests": [
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "size": 7143,
      "digest": "sha256:e692418e4cbaf90ca69d05a66403747baa33ee08806650b51fab815ad7fc331f",
      "platform": {
        "architecture": "ppc64le"
      },
      "annotations": {
        "name": "a_package_name",
        "version": "0.0.0"
      }
    },
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "size": 7682,
      "digest": "sha256:5b0bcabd1ed22e9fb1310cf6c2dec7cdef19f0ad69efa1f392e94a4333501270",
      "platform": {
        "architecture": "amd64"
      }
      "annotations": {
        "name": "a_package_name",
        "version": "0.0.0"
      }
    }
  ]
}
```

Underlying packages identified with `pkgid` where it could be either RPM checksum or package name/version checksum:

```
/<project|org_name>/yum/<repo_name>/packages:<pkgid>
```
