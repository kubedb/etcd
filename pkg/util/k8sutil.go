package util

import (
	"k8s.io/api/core/v1"
	"strings"
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

	etcdVersionAnnotationKey = "etcd.version"

	defaultDNSTimeout = int64(0)
	EtcdClientPort    = 2379
)

func GetEtcdVersion(pod *v1.Pod) string {
	img := pod.Spec.Containers[0].Image
	tv := strings.Split(img, ":")
	return tv[1]
}

func SetEtcdVersion(pod *v1.Pod, version string) {
	pod.Annotations[etcdVersionAnnotationKey] = version
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
