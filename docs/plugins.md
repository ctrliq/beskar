# Beskar Plugins

TODOS:
- [ ] Generate with Kun - See build/mage/build.go:86 for autogeneration
- [ ] Map artifact paths with a separator like `/artifacts/ostree/{repo_name}/separtor/path/to/{artifact_name}`.  This will be translated to `/v2/%s/blobs/sha256:%s`
- [ ] Create router.rego & data.json so that Beskar knows how to route requests to plugin server(s)
- [ ] mediatypes may be needed for each file type
    - [ ] `application/vnd.ciq.ostree.file.v1.file`
    - [ ] `application/vnd.ciq.ostree.summary.v1.summary`



See internal/plugins/yum/embedded/router.rego for example
/artifacts/ostree/{repo_name}/separtor/path/to/{artifact_name}

/2/artifacts/ostree/{repo_name}/files:summary
/2/artifacts/ostree/{repo_name}/files:{sha256("/path/to/{artifact_name}")}