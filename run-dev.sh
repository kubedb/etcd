#!/bin/sh

./hack/make.py 
./hack/dev/setup.sh

etcd run --docker-registry=sanjid \
                              --secure-port=8443 \
                              --kubeconfig="$HOME/.kube/config" \
                              --authorization-kubeconfig="$HOME/.kube/config" \
                              --authentication-kubeconfig="$HOME/.kube/config"
