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
package framework

import (
	"os"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

	"github.com/appscode/go/types"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	store "kmodules.xyz/objectstore-api/api/v1"
)

const (
	updateRetryInterval = 10 * 1000 * 1000 * time.Nanosecond
	maxAttempts         = 5
)

const (
	S3_BUCKET_NAME       = "S3_BUCKET_NAME"
	GCS_BUCKET_NAME      = "GCS_BUCKET_NAME"
	AZURE_CONTAINER_NAME = "AZURE_CONTAINER_NAME"
	SWIFT_CONTAINER_NAME = "SWIFT_CONTAINER_NAME"
)

func deleteInBackground() *metav1.DeleteOptions {
	policy := metav1.DeletePropagationBackground
	return &metav1.DeleteOptions{PropagationPolicy: &policy}
}

func deleteInForeground() *metav1.DeleteOptions {
	policy := metav1.DeletePropagationForeground
	return &metav1.DeleteOptions{PropagationPolicy: &policy}
}

func (f *Invocation) GetPersistentVolumeClaim() *core.PersistentVolumeClaim {
	return &core.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.app,
			Namespace: f.namespace,
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadWriteOnce,
			},
			StorageClassName: &f.StorageClass,
			Resources: core.ResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceName(core.ResourceStorage): resource.MustParse("50Mi"),
				},
			},
		},
	}
}

func (f *Invocation) CreatePersistentVolumeClaim(pvc *core.PersistentVolumeClaim) error {
	_, err := f.kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(pvc)
	return err
}

func (f *Invocation) DeletePersistentVolumeClaim(meta metav1.ObjectMeta) error {
	return f.kubeClient.CoreV1().PersistentVolumeClaims(meta.Namespace).Delete(meta.Name, deleteInForeground())
}

func (f *Invocation) LocalStorageSpec() *store.LocalSpec {
	return &store.LocalSpec{
		MountPath: "/repo",
		VolumeSource: core.VolumeSource{
			EmptyDir: &core.EmptyDirVolumeSource{},
		},
	}
}

func (f *Invocation) EtcdPVCSpec() *core.PersistentVolumeClaimSpec {
	return &core.PersistentVolumeClaimSpec{
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceStorage: resource.MustParse("1Gi"),
			},
		},
		StorageClassName: types.StringP(f.StorageClass),
	}
}

func (f *Invocation) LocalBackupScheduleSpec(secretName string) *api.BackupScheduleSpec {
	return &api.BackupScheduleSpec{
		CronExpression: "@every 1m",
		Backend: store.Backend{
			StorageSecretName: secretName,
			Local: &store.LocalSpec{
				MountPath: "/repo",
				VolumeSource: core.VolumeSource{
					EmptyDir: &core.EmptyDirVolumeSource{},
				},
			},
		},
	}
}

func (f *Invocation) GCSBackupScheduleSpec(secretName string) *api.BackupScheduleSpec {
	return &api.BackupScheduleSpec{
		CronExpression: "@every 1m",
		Backend: store.Backend{
			StorageSecretName: secretName,
			GCS: &store.GCSSpec{
				Bucket: os.Getenv(GCS_BUCKET_NAME),
			},
		},
	}
}
