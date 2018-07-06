package etcd_helper

import (
	"fmt"
	"log"
	"os"
	"strings"

//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/client-go/kubernetes"
	//restclient "k8s.io/client-go/rest"
	"github.com/kubedb/etcd/pkg/etcdmain"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/clientv3"
	"time"
	"context"
	"os/exec"
)

func RunEtcdHelper(etcdConf *etcdmain.Config) {

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}
	fmt.Println(etcdConf)

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}
	parts := strings.Split(hostname, "-")
	statefulSetName := strings.Join(parts[:len(parts)-1], "-")

	leader := fmt.Sprintf("%s-0", statefulSetName)


	clusterState := "new"
	if hostname != leader {
		clusterState = "existing"
		clusters := etcdConf.InitialCluster
		urls, err := types.NewURLsMap(clusters)
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(urls, "................")
		endpoints := make([]string, 0)
		for _, url := range urls {
			hostport := strings.Split(url[0].Host, ":")
			endpoints = append(endpoints, fmt.Sprintf("%s://%s:2379", url[0].Scheme, hostport[0]))

		}

		fmt.Println(endpoints)
		cfg := clientv3.Config{
			Endpoints:  []string{endpoints[0]},
			DialTimeout: 5 * time.Second,
			//TLS:         c.tlsConfig,
		}
		etcdcli, err := clientv3.New(cfg)
		if err != nil {
			log.Fatalln(err)
		}
		defer etcdcli.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
		resp, err := etcdcli.MemberAdd(ctx, []string{etcdConf.APUrls[0].String()})
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(resp.Members)
		cancel()


	}

	commands := fmt.Sprintf("--data-dir=%s --name=%s --initial-advertise-peer-urls=%s "+
		"--listen-peer-urls=%v --listen-client-urls=%v --advertise-client-urls=%v "+
		"--initial-cluster=%s --initial-cluster-state=%s",
		etcdConf.Dir, etcdConf.Name, etcdConf.APUrls[0].String(), etcdConf.LPUrls[0].String(), etcdConf.LCUrls[0].String(), etcdConf.ACUrls[0].String(),
			etcdConf.InitialCluster, clusterState)

	/*if m.SecurePeer {
		commands += fmt.Sprintf(" --peer-client-cert-auth=true --peer-trusted-ca-file=%[1]s/peer-ca.crt --peer-cert-file=%[1]s/peer.crt --peer-key-file=%[1]s/peer.key", peerTLSDir)
	}
	if m.SecureClient {
		commands += fmt.Sprintf(" --client-cert-auth=true --trusted-ca-file=%[1]s/server-ca.crt --cert-file=%[1]s/server.crt --key-file=%[1]s/server.key", serverTLSDir)
	}*/
	if clusterState == "new" {
		commands = fmt.Sprintf("%s --initial-cluster-token=%s", commands, statefulSetName)
	}
	fmt.Println(commands, "<><><>>>>>")
	cmd:= exec.Command("/usr/local/bin/etcd", commands)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Run(); err != nil {
		log.Println(err)
	}


}
