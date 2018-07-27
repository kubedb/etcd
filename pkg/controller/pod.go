package controller

import (
	"fmt"
	"strings"

	"github.com/appscode/go/log"
	mon_api "github.com/appscode/kube-mon/api"
	"github.com/appscode/kutil"
	core_util "github.com/appscode/kutil/core/v1"
	meta_util "github.com/appscode/kutil/meta"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/etcd/pkg/util"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
)

const (
	// EtcdClientPort is the client port on client service and etcd nodes.
	EtcdClientPort = 2379

	etcdVolumeMountDir    = "/var/etcd"
	dataDir               = etcdVolumeMountDir + "/data"
	peerTLSDir            = "/etc/etcdtls/member/peer-tls"
	peerTLSVolume         = "member-peer-tls"
	serverTLSDir          = "/etc/etcdtls/member/server-tls"
	serverTLSVolume       = "member-server-tls"
	operatorEtcdTLSDir    = "/etc/etcdtls/operator/etcd-tls"
	operatorEtcdTLSVolume = "etcd-client-tls"
	ExporterSecretPath    = "/var/run/secrets/kubedb.com/"
)

func (c *Controller) createPod(cluster *api.Etcd, members util.MemberSet, m *util.Member, state string) (*core.Pod, kutil.VerbType, error) {
	initialCluster := members.PeerURLPairs()
	podMeta := metav1.ObjectMeta{
		Name:      m.Name,
		Namespace: m.Namespace,
	}
	token := cluster.Name

	ref, rerr := reference.GetReference(clientsetscheme.Scheme, cluster)
	if rerr != nil {
		return nil, kutil.VerbUnchanged, rerr
	}

	initContainers := []core.Container{
		{
			Image: "busybox:1.28.0-glibc",
			Name:  "check-dns",
			Command: []string{"/bin/sh", "-c", fmt.Sprintf(`
					while ( ! nslookup %s )
					do
						sleep 1
					done`, m.Addr())},
		},
	}
	osmVolume := core.Volume{}
	hasOsmVolume := false
	if _, err := meta_util.GetString(cluster.Annotations, api.AnnotationInitialized); err == kutil.ErrNotFound &&
		cluster.Spec.Init != nil && cluster.Spec.Init.SnapshotSource != nil {

		if err := c.initialize(cluster); err != nil {
			return nil, kutil.VerbUnchanged, err
		}
		snapshotSource := cluster.Spec.Init.SnapshotSource
		snapshot, err := c.Controller.ExtClient.Snapshots(cluster.Namespace).Get(snapshotSource.Name, metav1.GetOptions{})
		if err != nil {
			return nil, kutil.VerbUnchanged, err
		}

		restore, err := c.getRestoreContainer(cluster, snapshot, m, members)
		if err != nil {
			return nil, kutil.VerbUnchanged, err
		}
		initContainers = append(initContainers, restore...)

		if err = c.createOsmSecret(snapshot); err != nil {
			return nil, kutil.VerbUnchanged, err
		}
		osmVolume = core.Volume{
			Name: "osmconfig",
			VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{
					SecretName: snapshot.OSMSecretName(),
				},
			},
		}
		hasOsmVolume = true
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

	return core_util.CreateOrPatchPod(c.Controller.Client, podMeta, func(in *core.Pod) *core.Pod {
		in.ObjectMeta = core_util.EnsureOwnerReference(in.ObjectMeta, ref)
		in.Labels = core_util.UpsertMap(in.Labels, cluster.StatefulSetLabels())
		in.Annotations = core_util.UpsertMap(in.Annotations, map[string]string{
			//	"etcd.version": c.cluster.Spec.Version,
		})

		livenessProbe := newEtcdProbe(false)
		readinessProbe := newEtcdProbe(false)
		readinessProbe.InitialDelaySeconds = 1
		readinessProbe.TimeoutSeconds = 5
		readinessProbe.PeriodSeconds = 5
		readinessProbe.FailureThreshold = 3
		container := core.Container{
			Name:            api.ResourceSingularEtcd,
			Image:           c.docker.GetImageWithTag(cluster),
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
			Resources: cluster.Spec.Resources,
		}
		volumes := []core.Volume{}
		if m.SecurePeer {
			container.VolumeMounts = append(container.VolumeMounts, core.VolumeMount{
				MountPath: peerTLSDir,
				Name:      peerTLSVolume,
			})
			volumes = append(volumes, core.Volume{Name: peerTLSVolume, VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{SecretName: cluster.Spec.TLS.Member.PeerSecret},
			}})
		}
		if m.SecureClient {
			container.VolumeMounts = append(container.VolumeMounts, core.VolumeMount{
				MountPath: serverTLSDir,
				Name:      serverTLSVolume,
			}, core.VolumeMount{
				MountPath: operatorEtcdTLSDir,
				Name:      operatorEtcdTLSVolume,
			})
			volumes = append(volumes, core.Volume{Name: serverTLSVolume, VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{SecretName: cluster.Spec.TLS.Member.ServerSecret},
			}}, core.Volume{Name: operatorEtcdTLSVolume, VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{SecretName: cluster.Spec.TLS.OperatorSecret},
			}})
		}

		if hasOsmVolume {
			volumes = append(volumes, osmVolume)
		}

		in.Spec.Containers = core_util.UpsertContainer(in.Spec.Containers, container)
		in.Spec.Volumes = core_util.UpsertVolume(in.Spec.Volumes, volumes...)

		if cluster.GetMonitoringVendor() == mon_api.VendorPrometheus {
			in.Spec.Containers = core_util.UpsertContainer(in.Spec.Containers, core.Container{
				Name: "exporter",
				Args: append([]string{
					"export",
					fmt.Sprintf("--address=:%d", cluster.Spec.Monitor.Prometheus.Port),
					fmt.Sprintf("--enable-analytics=%v", c.EnableAnalytics),
				}, c.LoggerOptions.ToFlags()...),
				Image: c.docker.GetOperatorImageWithTag(cluster),
				Ports: []core.ContainerPort{
					{
						Name:          api.PrometheusExporterPortName,
						Protocol:      core.ProtocolTCP,
						ContainerPort: cluster.Spec.Monitor.Prometheus.Port,
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
			in.Spec.Volumes = core_util.UpsertVolume(
				in.Spec.Volumes,
				core.Volume{
					Name: "db-secret",
					VolumeSource: core.VolumeSource{
						Secret: &core.SecretVolumeSource{
							SecretName: cluster.Spec.DatabaseSecret.SecretName,
						},
					},
				},
			)
		}

		in.Spec.RestartPolicy = core.RestartPolicyNever
		in.Spec.Hostname = m.Name
		in.Spec.Subdomain = cluster.Name
		in.Spec.AutomountServiceAccountToken = func(b bool) *bool { return &b }(false)

		createPodPvc(c.Controller.Client, m, cluster)
		in = upsertEnv(in, cluster)
		in = upsertDataVolume(in, cluster)

		in.Spec.InitContainers = append(in.Spec.InitContainers, initContainers...)

		return in
	})

}

func newEtcdProbe(isSecure bool) *core.Probe {
	// etcd pod is alive only if a linearizable get succeeds.
	cmd := "ETCDCTL_API=3 etcdctl get foo"
	if isSecure {
		tlsFlags := fmt.Sprintf("--cert=%[1]s/%[2]s --key=%[1]s/%[3]s --cacert=%[1]s/%[4]s", operatorEtcdTLSDir, etcdutil.CliCertFile, etcdutil.CliKeyFile, etcdutil.CliCAFile)
		cmd = fmt.Sprintf("ETCDCTL_API=3 etcdctl --endpoints=https://localhost:%d %s get foo", EtcdClientPort, tlsFlags)
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
				MountPath: etcdVolumeMountDir,
			}
			volumeMounts := container.VolumeMounts
			volumeMounts = core_util.UpsertVolumeMount(volumeMounts, volumeMount)
			pod.Spec.Containers[i].VolumeMounts = volumeMounts

			pvcSpec := etcd.Spec.Storage
			volume := core.Volume{
				Name: "data",
			}
			if pvcSpec != nil {
				if len(pvcSpec.AccessModes) == 0 {
					pvcSpec.AccessModes = []core.PersistentVolumeAccessMode{
						core.ReadWriteOnce,
					}
					log.Infof(`Using "%v" as AccessModes in etcd.Spec.Storage`, core.ReadWriteOnce)
				}

				volume.VolumeSource = core.VolumeSource{
					PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
						ClaimName: pod.Name,
					},
				}
			} else {
				volume.VolumeSource = core.VolumeSource{
					EmptyDir: &core.EmptyDirVolumeSource{},
				}
			}

			volumeClaims := pod.Spec.Volumes
			volumeClaims = core_util.UpsertVolume(volumeClaims, volume)
			pod.Spec.Volumes = volumeClaims
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

func createPodPvc(client kubernetes.Interface, m *util.Member, etcd *api.Etcd) (*core.PersistentVolumeClaim, error) {
	if etcd.Spec.Storage != nil {
		pvc := &core.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      m.Name,
				Namespace: etcd.Namespace,
				Labels: map[string]string{
					"etcd_cluster": etcd.Name,
					"app":          "etcd",
				},
			},
			Spec: *etcd.Spec.Storage,
		}
		ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd)
		if rerr != nil {
			return nil, rerr
		}
		pvc.ObjectMeta = core_util.EnsureOwnerReference(pvc.ObjectMeta, ref)
		return client.CoreV1().PersistentVolumeClaims(etcd.Namespace).Create(pvc)
	}
	return nil, nil
}
