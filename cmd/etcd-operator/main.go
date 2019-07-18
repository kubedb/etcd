package main

import (
	"log"

	"kmodules.xyz/client-go/logs"
	"kubedb.dev/etcd/pkg/cmds"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()
	if err := cmds.NewRootCmd(Version).Execute(); err != nil {
		log.Fatal(err)
	}
}
