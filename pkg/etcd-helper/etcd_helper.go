/*
Copyright The KubeDB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package etcd_helper

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"kubedb.dev/etcd/pkg/etcdmain"
	"kubedb.dev/etcd/pkg/util"

	"github.com/coreos/etcd/clientv3"
)

func RunEtcdHelper(etcdConf *etcdmain.Config) {
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}
	parts := strings.Split(hostname, "-")
	statefulSetName := strings.Join(parts[:len(parts)-1], "-")

	leaderName := fmt.Sprintf("%s-0", statefulSetName)
	governingService := fmt.Sprintf("%s-gvs", statefulSetName)

	ms := util.NewMemberSet()
	replica, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		log.Fatalln(err)
	}
	for i := 0; i <= replica; i++ {
		podName := fmt.Sprintf("%s-%v", statefulSetName, i)
		member := util.NewMember(podName, namespace, governingService)
		ms.Add(member)
	}

	clusterState := "new"
	if hostname != leaderName {
		clusterState = "existing"

		fmt.Println(ms.ClientURLs())
		cfg := clientv3.Config{
			Endpoints:   ms.ClientURLs(),
			DialTimeout: util.DefaultTimeoutSecond,
			//TLS:         c.tlsConfig,
		}
		etcdcli, err := clientv3.New(cfg)
		if err != nil {
			log.Fatalln(err)
		}
		defer etcdcli.Close()

		ctx, cancel := context.WithTimeout(context.Background(), util.DefaultTimeoutSecond)
		resp, err := etcdcli.MemberAdd(ctx, []string{ms[hostname].PeerURL()})
		fmt.Println(resp)
		if err != nil {
			log.Fatalln(err)
		}
		cancel()

	}

	leader := util.NewMember(hostname, namespace, governingService)

	args := leader.BuildEtcdArgs()
	args = append(args, fmt.Sprintf("--initial-cluster=%s", strings.Join(ms.PeerURLPairs(), ",")))
	args = append(args, fmt.Sprintf("--initial-cluster-state=%s", clusterState))

	/*if m.SecurePeer {
		commands += fmt.Sprintf(" --peer-client-cert-auth=true --peer-trusted-ca-file=%[1]s/peer-ca.crt --peer-cert-file=%[1]s/peer.crt --peer-key-file=%[1]s/peer.key", peerTLSDir)
	}
	if m.SecureClient {
		commands += fmt.Sprintf(" --client-cert-auth=true --trusted-ca-file=%[1]s/server-ca.crt --cert-file=%[1]s/server.crt --key-file=%[1]s/server.key", serverTLSDir)
	}*/
	if clusterState == "new" {
		args = append(args, fmt.Sprintf("--initial-cluster-token=%s", statefulSetName))
	}

	fmt.Println(args, "###############################")

	cmd := exec.Command("/usr/local/bin/etcd", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Run(); err != nil {
		log.Println(err)
	}

	select {}

}
