package controller

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/appscode/kutil/tools/analytics"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/pkg/eventer"
	"github.com/kubedb/apimachinery/pkg/storage"
	etcdutil "github.com/kubedb/etcd/pkg/util"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
)

const (
	snapshotDumpDir        = etcdVolumeMountDir + "/snapshot"
	snapshotProcessRestore = "restore"
	snapshotProcessBackup  = "backup"
)

func (c *Controller) getRestoreContainer(etcd *api.Etcd, snapshot *api.Snapshot, m *etcdutil.Member, ms etcdutil.MemberSet) ([]core.Container, error) {
	snapshotSource := etcd.Spec.Init.SnapshotSource
	// Event for notification that kubernetes objects are creating
	if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
		c.recorder.Eventf(
			ref,
			core.EventTypeNormal,
			eventer.EventReasonInitializing,
			`Initializing from Snapshot: "%v"`,
			snapshotSource.Name,
		)
	}
	containers := []core.Container{}

	namespace := snapshotSource.Namespace
	if namespace == "" {
		namespace = etcd.Namespace
	}

	endpoints := fmt.Sprintf("%s.%s", ClientServiceName(etcd.Name), etcd.Namespace)
	backupSpec := snapshot.Spec.SnapshotStorageSpec
	bucket, err := backupSpec.Container()
	if err != nil {
		return nil, err
	}
	folderName, _ := snapshot.Location()

	containers = append(containers, core.Container{
		Name:  snapshotProcessRestore,
		Image: c.docker.GetToolsImageWithTag(etcd),
		Args: []string{
			snapshotProcessRestore,
			fmt.Sprintf(`--host=%s`, endpoints),
			fmt.Sprintf(`--data-dir=%s`, snapshotDumpDir),
			fmt.Sprintf(`--bucket=%s`, bucket),
			fmt.Sprintf(`--folder=%s`, folderName),
			fmt.Sprintf(`--snapshot=%s`, snapshot.Name),
			fmt.Sprintf(`--enable-analytics=%v`, c.EnableAnalytics),
		},
		Env: []core.EnvVar{
			{
				Name:  analytics.Key,
				Value: c.AnalyticsClientID,
			},
		},
		Resources: snapshot.Spec.Resources,
		VolumeMounts: []core.VolumeMount{
			{
				Name:      "data",
				MountPath: etcdVolumeMountDir,
			},
			{
				Name:      "osmconfig",
				MountPath: storage.SecretMountPath,
				ReadOnly:  true,
			},
		},
	})

	containers = append(containers, core.Container{
		Name:  "restore-datadir",
		Image: c.docker.GetToolsImageWithTag(etcd),
		Command: []string{
			"/bin/sh", "-ec",
			fmt.Sprintf("ETCDCTL_API=3 etcdctl snapshot restore %[1]s"+
				" --name %[2]s"+
				" --initial-cluster %[3]s"+
				" --initial-cluster-token %[5]s"+
				" --initial-advertise-peer-urls %[4]s"+
				" --data-dir %[6]s 2>/dev/termination-log", filepath.Join(snapshotDumpDir, snapshot.Name), m.Name, strings.Join(ms.PeerURLPairs(), ","), m.PeerURL(), etcd.Name, dataDir),
		},
		Resources: snapshot.Spec.Resources,
		VolumeMounts: []core.VolumeMount{
			{
				Name:      "data",
				MountPath: etcdVolumeMountDir,
			},
		},
	})
	fmt.Println(containers)

	return containers, nil
}

func (c *Controller) getSnapshotterJob(snapshot *api.Snapshot) (*batch.Job, error) {
	databaseName := snapshot.Spec.DatabaseName
	jobName := fmt.Sprintf("%s-%s", api.DatabaseNamePrefix, snapshot.OffshootName())
	jobLabel := map[string]string{
		api.LabelDatabaseKind: api.ResourceKindEtcd,
	}
	jobAnnotation := map[string]string{
		api.AnnotationJobType: api.JobTypeBackup,
	}
	backupSpec := snapshot.Spec.SnapshotStorageSpec
	bucket, err := backupSpec.Container()
	if err != nil {
		return nil, err
	}
	etcd, err := c.etcdLister.Etcds(snapshot.Namespace).Get(databaseName)
	if err != nil {
		return nil, err
	}

	// Get PersistentVolume object for Backup Util pod.
	persistentVolume, err := c.getVolumeForSnapshot(etcd.Spec.Storage, jobName, snapshot.Namespace)
	if err != nil {
		return nil, err
	}

	endpoints := fmt.Sprintf("%s.%s", ClientServiceName(etcd.Name), etcd.Namespace)
	// Folder name inside Cloud bucket where backup will be uploaded
	folderName, _ := snapshot.Location()

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        jobName,
			Labels:      jobLabel,
			Annotations: jobAnnotation,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: api.SchemeGroupVersion.String(),
					Kind:       api.ResourceKindSnapshot,
					Name:       snapshot.Name,
					UID:        snapshot.UID,
				},
			},
		},
		Spec: batch.JobSpec{
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: jobLabel,
				},
				Spec: core.PodSpec{
					Containers: []core.Container{
						{
							Name:  snapshotProcessBackup,
							Image: c.docker.GetToolsImageWithTag(etcd),
							Args: []string{
								snapshotProcessBackup,
								fmt.Sprintf(`--host=%s`, endpoints),
								//fmt.Sprintf(`--user=%s`, etcdUser),
								fmt.Sprintf(`--data-dir=%s`, snapshotDumpDir),
								fmt.Sprintf(`--bucket=%s`, bucket),
								fmt.Sprintf(`--folder=%s`, folderName),
								fmt.Sprintf(`--snapshot=%s`, snapshot.Name),
								fmt.Sprintf(`--enable-analytics=%v`, c.EnableAnalytics),
							},
							Env: []core.EnvVar{
								{
									Name:  analytics.Key,
									Value: c.AnalyticsClientID,
								},
							},
							Resources: snapshot.Spec.Resources,
							VolumeMounts: []core.VolumeMount{
								{
									Name:      persistentVolume.Name,
									MountPath: snapshotDumpDir,
								},
								{
									Name:      "osmconfig",
									ReadOnly:  true,
									MountPath: storage.SecretMountPath,
								},
							},
						},
					},
					ImagePullSecrets: etcd.Spec.ImagePullSecrets,
					Volumes: []core.Volume{
						{
							Name:         persistentVolume.Name,
							VolumeSource: persistentVolume.VolumeSource,
						},
						{
							Name: "osmconfig",
							VolumeSource: core.VolumeSource{
								Secret: &core.SecretVolumeSource{
									SecretName: snapshot.OSMSecretName(),
								},
							},
						},
					},
					RestartPolicy: core.RestartPolicyNever,
				},
			},
		},
	}
	if snapshot.Spec.SnapshotStorageSpec.Local != nil {
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, core.VolumeMount{
			Name:      "local",
			MountPath: snapshot.Spec.SnapshotStorageSpec.Local.MountPath,
			SubPath:   snapshot.Spec.SnapshotStorageSpec.Local.SubPath,
		})
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, core.Volume{
			Name:         "local",
			VolumeSource: snapshot.Spec.SnapshotStorageSpec.Local.VolumeSource,
		})
	}
	return job, nil
}
