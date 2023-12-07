package router

import future.keywords.if
import future.keywords.in

default output = {"repository": "", "redirect_url": "", "found": false}

filename_checksum(filename) = checksum if {
	filename in data.tags
	checksum := filename
} else = checksum {
	checksum := crypto.md5(filename)
}

blob_url(repo, filename) = url {
	digest := oci.blob_digest(sprintf("%s:%s", [repo, filename_checksum(filename)]), "mediatype", data.mediatype.file)
    url := {
        "url": sprintf("/v2/%s/blobs/sha256:%s", [repo, digest]),
        "found": digest != "",
    }
}

output = obj {
    some index
    input.method in data.routes[index].methods
    match := regex.find_all_string_submatch_n(
        data.routes[index].pattern,
        input.path,
        1
    )[0]
    redirect := blob_url(
        sprintf("%s/files", [match[1]]),
        match[2],
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
    repo := object.get({}, data.routes[index].body_key, "")
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