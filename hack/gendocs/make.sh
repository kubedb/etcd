#!/usr/bin/env bash

pushd $GOPATH/src/kubedb.dev/etcd/hack/gendocs
go run main.go
popd
