#!/bin/bash
set -xeou pipefail

DOCKER_REGISTRY=${DOCKER_REGISTRY:-kubedb}
IMG=etcd
TAG=3.2.13
ORGINAL=quay.io/coreos/etcd:v3.2.13
docker pull $ORGINAL

docker tag $ORGINAL "$DOCKER_REGISTRY/$IMG:$TAG"
docker push "$DOCKER_REGISTRY/$IMG:$TAG"
