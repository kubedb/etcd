package util

import (
	"k8s.io/api/core/v1"
)

const (
	etcdVolumeMountDir    = "/var/db"
	dataDir               = etcdVolumeMountDir + "/data"
	operatorEtcdTLSDir    = "/etc/etcdtls/operator/etcd-tls"
	etcdVolumeName        = "etcd-data"
	peerTLSDir            = "/etc/etcdtls/member/peer-tls"
	peerTLSVolume         = "member-peer-tls"
	serverTLSDir          = "/etc/etcdtls/member/server-tls"
	serverTLSVolume       = "member-server-tls"
	operatorEtcdTLSVolume = "etcd-client-tls"

	EtcdVersionAnnotationKey = "etcd.version"

	defaultDNSTimeout = int64(0)
	EtcdClientPort    = 2379
)

func GetEtcdVersion(pod *v1.Pod) string {
	return pod.Annotations[EtcdVersionAnnotationKey]
}

func GetPodNames(pods []*v1.Pod) []string {
	if len(pods) == 0 {
		return nil
	}
	res := []string{}
	for _, p := range pods {
		res = append(res, p.Name)
	}
	return res
}
