package cluster

import (
	"fmt"
	"strings"

	"github.com/appscode/go/log"
	"github.com/appscode/kutil"
	core_util "github.com/appscode/kutil/core/v1"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/etcd/pkg/util"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
)

const (
	// EtcdClientPort is the client port on client service and etcd nodes.
	EtcdClientPort = 2379

	etcdVolumeMountDir = "/var/etcd"
	dataDir            = etcdVolumeMountDir + "/data"
	peerTLSDir         = "/etc/etcdtls/member/peer-tls"
	peerTLSVolume      = "member-peer-tls"
	serverTLSDir       = "/etc/etcdtls/member/server-tls"
	serverTLSVolume    = "member-server-tls"

	defaultDNSTimeout = int64(0)
)

func (c *Cluster) createPod(members util.MemberSet, m *util.Member, state string) (*core.Pod, kutil.VerbType, error) {
	initialCluster := members.PeerURLPairs()
	podMeta := metav1.ObjectMeta{
		Name:      m.Name,
		Namespace: m.Namespace,
	}
	token := c.cluster.Name

	ref, rerr := reference.GetReference(clientsetscheme.Scheme, c.cluster)
	if rerr != nil {
		return nil, kutil.VerbUnchanged, rerr
	}

	commands := fmt.Sprintf("/usr/local/bin/etcd --data-dir=%s --name=%s --initial-advertise-peer-urls=%s "+
		"--listen-peer-urls=%s --listen-client-urls=%s --advertise-client-urls=%s "+
		"--initial-cluster=%s --initial-cluster-state=%s",
		dataDir, m.Name, m.PeerURL(), m.ListenPeerURL(), m.ListenClientURL(), m.ClientURL(), strings.Join(initialCluster, ","), state)
	if m.SecurePeer {
		commands += fmt.Sprintf(" --peer-client-cert-auth=true --peer-trusted-ca-file=%[1]s/peer-ca.crt --peer-cert-file=%[1]s/peer.crt --peer-key-file=%[1]s/peer.key", peerTLSDir)
	}
	if m.SecureClient {
		commands += fmt.Sprintf(" --client-cert-auth=true --trusted-ca-file=%[1]s/server-ca.crt --cert-file=%[1]s/server.crt --key-file=%[1]s/server.key", serverTLSDir)
	}
	if state == "new" {
		commands = fmt.Sprintf("%s --initial-cluster-token=%s", commands, token)
	}

	DNSTimeout := defaultDNSTimeout

	return core_util.CreateOrPatchPod(c.config.KubeCli, podMeta, func(in *core.Pod) *core.Pod {
		in.ObjectMeta = core_util.EnsureOwnerReference(in.ObjectMeta, ref)
		in.Labels = core_util.UpsertMap(in.Labels, c.cluster.StatefulSetLabels())
		in.Labels = core_util.UpsertMap(in.Labels, map[string]string{
			"app":          "etcd",
			"etcd_node":    m.Name,
			"etcd_cluster": c.cluster.Name,
		})
		in.Annotations = core_util.UpsertMap(in.Annotations, map[string]string{
			//"etcd.version": c.cluster.Spec.Version
		})

		livenessProbe := newEtcdProbe(false)
		readinessProbe := newEtcdProbe(false)
		readinessProbe.InitialDelaySeconds = 1
		readinessProbe.TimeoutSeconds = 5
		readinessProbe.PeriodSeconds = 5
		readinessProbe.FailureThreshold = 3
		in.Spec.Containers = core_util.UpsertContainer(in.Spec.Containers, core.Container{
			Name:            api.ResourceSingularEtcd,
			Image:           c.config.docker.GetImageWithTag(c.cluster),
			ImagePullPolicy: core.PullAlways,
			Command:         strings.Split(commands, " "),
			LivenessProbe:   livenessProbe,
			ReadinessProbe:  readinessProbe,
			Ports: []core.ContainerPort{
				{
					Name:          "server",
					ContainerPort: int32(2380),
					Protocol:      core.ProtocolTCP,
				},
				{
					Name:          "client",
					ContainerPort: int32(2379),
					Protocol:      core.ProtocolTCP,
				},
			},
			Resources: c.cluster.Spec.Resources,
		})
		/*if c.cluster.GetMonitoringVendor() == mon_api.VendorPrometheus {
			in.Spec.Containers = core_util.UpsertContainer(in.Spec.Containers, core.Container{
				Name: "exporter",
				Args: append([]string{
					"export",
					fmt.Sprintf("--address=:%d", c.cluster.Spec.Monitor.Prometheus.Port),
					fmt.Sprintf("--enable-analytics=%v", c.EnableAnalytics),
				}, c.LoggerOptions.ToFlags()...),
				Image: c.docker.GetOperatorImageWithTag(etcd),
				Ports: []core.ContainerPort{
					{
						Name:          api.PrometheusExporterPortName,
						Protocol:      core.ProtocolTCP,
						ContainerPort: c.cluster.Spec.Monitor.Prometheus.Port,
					},
				},
				VolumeMounts: []core.VolumeMount{
					{
						Name:      "db-secret",
						MountPath: ExporterSecretPath,
						ReadOnly:  true,
					},
				},
			})
			in.Spec.Template.Spec.Volumes = core_util.UpsertVolume(
				in.Spec.Template.Spec.Volumes,
				core.Volume{
					Name: "db-secret",
					VolumeSource: core.VolumeSource{
						Secret: &core.SecretVolumeSource{
							SecretName: etcd.Spec.DatabaseSecret.SecretName,
						},
					},
				},
			)
		}*/
		in = upsertEnv(in, c.cluster)
		in = upsertDataVolume(in, c.cluster)

		in.Spec.Containers = core_util.UpsertContainer(in.Spec.Containers, core.Container{
			Image: "busybox:1.28.0-glibc",
			Name:  "check-dns",
			Command: []string{"/bin/sh", "-c", fmt.Sprintf(`
					TIMEOUT_READY=%d
					while ( ! nslookup %s )
					do
						# If TIMEOUT_READY is 0 we should never time out and exit
						TIMEOUT_READY=$(( TIMEOUT_READY-1 ))
                        if [ $TIMEOUT_READY -eq 0 ];
				        then
				            echo "Timed out waiting for DNS entry"
				            exit 1
				        fi
						sleep 1
					done`, DNSTimeout, m.Addr())},
		})

		return in
	})

}

func newEtcdProbe(isSecure bool) *core.Probe {
	// etcd pod is alive only if a linearizable get succeeds.
	cmd := "ETCDCTL_API=3 etcdctl get foo"
	if isSecure {
		//tlsFlags := fmt.Sprintf("--cert=%[1]s/%[2]s --key=%[1]s/%[3]s --cacert=%[1]s/%[4]s", operatorEtcdTLSDir, etcdutil.CliCertFile, etcdutil.CliKeyFile, etcdutil.CliCAFile)
		//cmd = fmt.Sprintf("ETCDCTL_API=3 etcdctl --endpoints=https://localhost:%d %s get foo", EtcdClientPort, tlsFlags)
	}
	return &core.Probe{
		Handler: core.Handler{
			Exec: &core.ExecAction{
				Command: []string{"/bin/sh", "-ec", cmd},
			},
		},
		InitialDelaySeconds: 10,
		TimeoutSeconds:      10,
		PeriodSeconds:       60,
		FailureThreshold:    3,
	}
}

func upsertDataVolume(pod *core.Pod, etcd *api.Etcd) *core.Pod {
	for i, container := range pod.Spec.Containers {
		if container.Name == api.ResourceSingularEtcd {
			volumeMount := core.VolumeMount{
				Name:      "data",
				MountPath: "/data/db",
			}
			volumeMounts := container.VolumeMounts
			volumeMounts = core_util.UpsertVolumeMount(volumeMounts, volumeMount)
			pod.Spec.Containers[i].VolumeMounts = volumeMounts

			pvcSpec := etcd.Spec.Storage
			if pvcSpec != nil {
				if len(pvcSpec.AccessModes) == 0 {
					pvcSpec.AccessModes = []core.PersistentVolumeAccessMode{
						core.ReadWriteOnce,
					}
					log.Infof(`Using "%v" as AccessModes in etcd.Spec.Storage`, core.ReadWriteOnce)
				}

				volumeClaim := core.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: *pvcSpec,
				}
				if pvcSpec.StorageClassName != nil {
					volumeClaim.Annotations = map[string]string{
						"volume.beta.kubernetes.io/storage-class": *pvcSpec.StorageClassName,
					}
				}
			} else {
				volume := core.Volume{
					Name: "data",
					VolumeSource: core.VolumeSource{
						EmptyDir: &core.EmptyDirVolumeSource{},
					},
				}
				volumes := pod.Spec.Volumes
				volumes = core_util.UpsertVolume(volumes, volume)
				pod.Spec.Volumes = volumes
				return pod
			}
			break
		}
	}
	return pod
}

func upsertEnv(pod *core.Pod, etcd *api.Etcd) *core.Pod {
	envList := []core.EnvVar{
		{
			Name: "NAMESPACE",
			ValueFrom: &core.EnvVarSource{
				FieldRef: &core.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name:  "PRIMARY_HOST",
			Value: etcd.ServiceName(),
		},
		{
			Name: "NODE_NAME",
			ValueFrom: &core.EnvVarSource{
				FieldRef: &core.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
	}
	for i, container := range pod.Spec.Containers {
		if container.Name == api.ResourceSingularEtcd {
			pod.Spec.Containers[i].Env = core_util.UpsertEnvVars(container.Env, envList...)
			return pod
		}
	}
	return pod
}
