package router

import future.keywords.if
import future.keywords.in

default output = {"repository": "", "redirect_url": "", "found": false}

blob_url(repo, filename, blobtype) = url if {
    blobtype == "packages"
    digest := oci.blob_digest(sprintf("%s:%s", [repo, crypto.md5(filename)]), "mediatype", data.mediatype.rpm)
    url := {
        "url": sprintf("/v2/%s/blobs/sha256:%s", [repo, digest]),
        "found": digest != ""
    }
} else = url if {
    filename == "repomd.xml"
    digest := oci.blob_digest(sprintf("%s:repomdxml", [repo]), "mediatype", data.mediatype.repomd)
    url := {
        "url": sprintf("/v2/%s/blobs/sha256:%s", [repo, digest]),
        "found": digest != ""
    }
} else = url if {
    filename == "repomd.xml.asc"
    digest := oci.blob_digest(sprintf("%s:repomdxml", [repo]), "mediatype", data.mediatype.repomdasc)
    url := {
        "url": sprintf("/v2/%s/blobs/sha256:%s", [repo, digest]),
        "found": digest != ""
    }
} else = url {
    digest := oci.blob_digest(sprintf("%s:repomdxml", [repo]), "annotation", sprintf("org.opencontainers.image.title=%s", [filename]))
    url := {
        "url": sprintf("/v2/%s/blobs/sha256:%s", [repo, digest]),
        "found": digest != ""
    }
}

output = obj {
    some index
    data.routes[index].blobtype != ""
    input.method in data.routes[index].methods
    match := regex.find_all_string_submatch_n(
        data.routes[index].pattern,
        input.path,
        1
    )[0]
    redirect := blob_url(
        sprintf("%s/%s", [match[1], data.routes[index].blobtype]),
        match[2],
        data.routes[index].blobtype,
    )
    obj := {
        "repository": match[1],
        "redirect_url": redirect.url,
        "found": redirect.found
    }
} else = obj if {
    data.routes[index].body == true
    match := regex.find_all_string_submatch_n(
        data.routes[index].pattern,
        input.path,
        1
    )[0]
    repo := object.get(request.body(), data.routes[index].body_key, "")
    obj := {
        "repository": repo,
        "redirect_url": "",
        "found": repo != ""
    }
} else = obj {
    match := regex.find_all_string_submatch_n(
        data.routes[index].pattern,
        input.path,
        1
    )[0]
    obj := {
        "repository": "",
        "redirect_url": "",
        "found": true
    }
}