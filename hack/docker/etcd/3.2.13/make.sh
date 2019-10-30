#!/bin/bash

# Copyright The KubeDB Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -xeou pipefail

GOPATH=$(go env GOPATH)
REPO_ROOT=$GOPATH/src/kubedb.dev/etcd

source "$REPO_ROOT/hack/libbuild/common/lib.sh"
source "$REPO_ROOT/hack/libbuild/common/kubedb_image.sh"

IMG=etcd
TAG=3.2.13

DIST="$REPO_ROOT/dist"
mkdir -p "$DIST"

build_binary() {
  pushd $REPO_ROOT
  ./hack/builddeps.sh
  ./hack/make.py build etcd-operator
  popd
}

build_docker() {
  pushd "$REPO_ROOT/hack/docker/etcd/$TAG"

  # Copy etcd-operator
  cp "$DIST/etcd-operator/etcd-operator-alpine-amd64" etcd-operator
  chmod 755 etcd-operator

  local cmd="docker build --pull -t $DOCKER_REGISTRY/$IMG:$TAG ."
  echo $cmd; $cmd

  rm etcd-operator
  popd
}

build() {
  build_binary
  build_docker
}

binary_repo $@
