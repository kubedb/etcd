package controller

import (
	"fmt"

	"github.com/appscode/go/log"
	"github.com/appscode/go/types"
	mon_api "github.com/appscode/kube-mon/api"
	"github.com/appscode/kutil"
	app_util "github.com/appscode/kutil/apps/v1"
	core_util "github.com/appscode/kutil/core/v1"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/pkg/eventer"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
)

func (c *Controller) ensureStatefulSet(etcd *api.Etcd) (kutil.VerbType, error) {
	if err := c.checkStatefulSet(etcd); err != nil {
		return kutil.VerbUnchanged, err
	}

	// Create statefulSet for Etcd database
	statefulSet, vt, err := c.createStatefulSet(etcd, 1)
	if err != nil {
		return kutil.VerbUnchanged, err
	}

	// Check StatefulSet Pod status
	if vt != kutil.VerbUnchanged {
		if err := c.checkStatefulSetPodStatus(statefulSet); err != nil {
			if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
				c.recorder.Eventf(
					ref,
					core.EventTypeWarning,
					eventer.EventReasonFailedToStart,
					`Failed to CreateOrPatch StatefulSet. Reason: %v`,
					err,
				)
			}
			return kutil.VerbUnchanged, err
		}
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
			c.recorder.Eventf(
				ref,
				core.EventTypeNormal,
				eventer.EventReasonSuccessful,
				"Successfully %v StatefulSet",
				vt,
			)
		}
	}
	return vt, nil
}

func (c *Controller) checkStatefulSet(etcd *api.Etcd) error {
	// SatatefulSet for Etcd database
	statefulSet, err := c.Client.AppsV1().StatefulSets(etcd.Namespace).Get(etcd.OffshootName(), metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return nil
		}
		return err
	}

	if statefulSet.Labels[api.LabelDatabaseKind] != api.ResourceKindEtcd {
		return fmt.Errorf(`intended statefulSet "%v" already exists`, etcd.OffshootName())
	}

	return nil
}

func (c *Controller) createStatefulSet(
	etcd *api.Etcd,
	replicas int32) (*apps.StatefulSet, kutil.VerbType, error) {
	statefulSetMeta := metav1.ObjectMeta{
		Name:      etcd.OffshootName(),
		Namespace: etcd.Namespace,
	}

	ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd)
	if rerr != nil {
		return nil, kutil.VerbUnchanged, rerr
	}

	/*commands := fmt.Sprintf("/usr/local/bin/etcd --data-dir=%s --name=%s --initial-advertise-peer-urls=%s "+
		"--listen-peer-urls=%s --listen-client-urls=%s --advertise-client-urls=%s "+
		"--initial-cluster=%s --initial-cluster-state=%s",
		"/var/lib/etcd", etcd.Name, etcd.PeerURL(), m.ListenPeerURL(), m.ListenClientURL(), m.ClientURL(), strings.Join(initialCluster, ","), state)
	if m.SecurePeer {
		commands += fmt.Sprintf(" --peer-client-cert-auth=true --peer-trusted-ca-file=%[1]s/peer-ca.crt --peer-cert-file=%[1]s/peer.crt --peer-key-file=%[1]s/peer.key", peerTLSDir)
	}
	if m.SecureClient {
		commands += fmt.Sprintf(" --client-cert-auth=true --trusted-ca-file=%[1]s/server-ca.crt --cert-file=%[1]s/server.crt --key-file=%[1]s/server.key", serverTLSDir)
	}
	*/

	return app_util.CreateOrPatchStatefulSet(c.Client, statefulSetMeta, func(in *apps.StatefulSet) *apps.StatefulSet {
		in.ObjectMeta = core_util.EnsureOwnerReference(in.ObjectMeta, ref)
		in.Labels = core_util.UpsertMap(in.Labels, etcd.StatefulSetLabels())
		in.Annotations = core_util.UpsertMap(in.Annotations, etcd.StatefulSetAnnotations())

		in.Spec.Replicas = types.Int32P(replicas)
		in.Spec.ServiceName = c.GoverningService
		in.Spec.Template.Labels = in.Labels
		in.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: in.Labels,
		}

		livenessProbe := newEtcdProbe(false)
		readinessProbe := newEtcdProbe(false)
		readinessProbe.InitialDelaySeconds = 1
		readinessProbe.TimeoutSeconds = 5
		readinessProbe.PeriodSeconds = 5
		readinessProbe.FailureThreshold = 3

		in.Spec.Template.Spec.Containers = core_util.UpsertContainer(in.Spec.Template.Spec.Containers, core.Container{
			Name:            api.ResourceSingularEtcd,
			Image:           c.docker.GetImageWithTag(etcd),
			ImagePullPolicy: core.PullAlways,
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
			Resources: etcd.Spec.Resources,
		})
		if etcd.GetMonitoringVendor() == mon_api.VendorPrometheus {
			in.Spec.Template.Spec.Containers = core_util.UpsertContainer(in.Spec.Template.Spec.Containers, core.Container{
				Name: "exporter",
				Args: append([]string{
					"export",
					fmt.Sprintf("--address=:%d", etcd.Spec.Monitor.Prometheus.Port),
					fmt.Sprintf("--enable-analytics=%v", c.EnableAnalytics),
				}, c.LoggerOptions.ToFlags()...),
				Image: c.docker.GetOperatorImageWithTag(etcd),
				Ports: []core.ContainerPort{
					{
						Name:          api.PrometheusExporterPortName,
						Protocol:      core.ProtocolTCP,
						ContainerPort: etcd.Spec.Monitor.Prometheus.Port,
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
		}
		// Set Admin Secret as MYSQL_ROOT_PASSWORD env variable
		in = upsertEnv(in, etcd)
		in = upsertDataVolume(in, etcd)
		if etcd.Spec.Init != nil && etcd.Spec.Init.ScriptSource != nil {
			in = upsertInitScript(in, etcd.Spec.Init.ScriptSource.VolumeSource)
		}

		in.Spec.Template.Spec.NodeSelector = etcd.Spec.NodeSelector
		in.Spec.Template.Spec.Affinity = etcd.Spec.Affinity
		in.Spec.Template.Spec.Tolerations = etcd.Spec.Tolerations
		in.Spec.Template.Spec.ImagePullSecrets = etcd.Spec.ImagePullSecrets
		if etcd.Spec.SchedulerName != "" {
			in.Spec.Template.Spec.SchedulerName = etcd.Spec.SchedulerName
		}

		if c.EnableRBAC {
			in.Spec.Template.Spec.ServiceAccountName = etcd.OffshootName()
		}

		in.Spec.UpdateStrategy.Type = apps.RollingUpdateStatefulSetStrategyType
		return in
	})
}

func upsertDataVolume(statefulSet *apps.StatefulSet, etcd *api.Etcd) *apps.StatefulSet {
	for i, container := range statefulSet.Spec.Template.Spec.Containers {
		if container.Name == api.ResourceSingularEtcd {
			volumeMount := core.VolumeMount{
				Name:      "data",
				MountPath: "/data/db",
			}
			volumeMounts := container.VolumeMounts
			volumeMounts = core_util.UpsertVolumeMount(volumeMounts, volumeMount)
			statefulSet.Spec.Template.Spec.Containers[i].VolumeMounts = volumeMounts

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
				volumeClaims := statefulSet.Spec.VolumeClaimTemplates
				volumeClaims = core_util.UpsertVolumeClaim(volumeClaims, volumeClaim)
				statefulSet.Spec.VolumeClaimTemplates = volumeClaims
			} else {
				volume := core.Volume{
					Name: "data",
					VolumeSource: core.VolumeSource{
						EmptyDir: &core.EmptyDirVolumeSource{},
					},
				}
				volumes := statefulSet.Spec.Template.Spec.Volumes
				volumes = core_util.UpsertVolume(volumes, volume)
				statefulSet.Spec.Template.Spec.Volumes = volumes
				return statefulSet
			}
			break
		}
	}
	return statefulSet
}

func upsertEnv(statefulSet *apps.StatefulSet, etcd *api.Etcd) *apps.StatefulSet {
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
		/*{
			Name: "ETCD_INITDB_ROOT_USERNAME",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: etcd.Spec.DatabaseSecret.SecretName,
					},
					Key: KeyEtcdUser,
				},
			},
		},
		{
			Name: "ETCD_INITDB_ROOT_PASSWORD",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: etcd.Spec.DatabaseSecret.SecretName,
					},
					Key: KeyEtcdPassword,
				},
			},
		},*/
	}
	for i, container := range statefulSet.Spec.Template.Spec.Containers {
		if container.Name == api.ResourceSingularEtcd {
			statefulSet.Spec.Template.Spec.Containers[i].Env = core_util.UpsertEnvVars(container.Env, envList...)
			return statefulSet
		}
	}
	return statefulSet
}

func upsertInitScript(statefulSet *apps.StatefulSet, script core.VolumeSource) *apps.StatefulSet {
	for i, container := range statefulSet.Spec.Template.Spec.Containers {
		if container.Name == api.ResourceSingularEtcd {
			volumeMount := core.VolumeMount{
				Name:      "initial-script",
				MountPath: "/docker-entrypoint-initdb.d",
			}
			statefulSet.Spec.Template.Spec.Containers[i].VolumeMounts = core_util.UpsertVolumeMount(
				container.VolumeMounts,
				volumeMount,
			)

			volume := core.Volume{
				Name:         "initial-script",
				VolumeSource: script,
			}
			statefulSet.Spec.Template.Spec.Volumes = core_util.UpsertVolume(
				statefulSet.Spec.Template.Spec.Volumes,
				volume,
			)
			return statefulSet
		}
	}
	return statefulSet
}

func upsertObjectMeta(statefulSet *apps.StatefulSet, postgres *api.Postgres) *apps.StatefulSet {
	statefulSet.Labels = core_util.UpsertMap(statefulSet.Labels, postgres.StatefulSetLabels())
	statefulSet.Annotations = core_util.UpsertMap(statefulSet.Annotations, postgres.StatefulSetAnnotations())
	return statefulSet
}

func (c *Controller) checkStatefulSetPodStatus(statefulSet *apps.StatefulSet) error {
	err := core_util.WaitUntilPodRunningBySelector(
		c.Client,
		statefulSet.Namespace,
		statefulSet.Spec.Selector,
		int(types.Int32(statefulSet.Spec.Replicas)),
	)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) ensureCombinedNode(etcd *api.Etcd) (kutil.VerbType, error) {
	return c.ensureStatefulSet(etcd)
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
