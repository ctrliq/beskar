#!/bin/bash
set -euo pipefail

# This script generates test data for libostree.

# Clean up any existing test data
rm -rf testdata

mkdir -p testdata/{repo,tree}

# Create a simple ostree repo with two branches and a series of commits.
ostree --repo=testdata/repo init --mode=archive

echo "Test file in a simple ostree repo - branch test1" > ./testdata/tree/testfile.txt
ostree --repo=testdata/repo commit --branch=test1 ./testdata/tree/

echo "Test file in a simple ostree repo - branch test2" > ./testdata/tree/testfile.txt
ostree --repo=testdata/repo commit --branch=test2 ./testdata/tree/

echo "Another test file" > ./testdata/tree/another_testfile.txt
ostree --repo=testdata/repo commit --branch=test2 ./testdata/tree/

ostree --repo=testdata/repo summary --update

# We don't actually need the tree directory, just the repo.
rm -rf testdata/tree