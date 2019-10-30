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
	"fmt"

	api "kubedb.dev/apimachinery/apis/catalog/v1alpha1"

	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (i *Invocation) EtcdVersion() *api.EtcdVersion {
	return &api.EtcdVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: DBVersion,
			Labels: map[string]string{
				"app": i.app,
			},
		},
		Spec: api.EtcdVersionSpec{
			Version: DBVersion,
			DB: api.EtcdVersionDatabase{
				Image: fmt.Sprintf("%s/etcd:%s", DockerRegistry, DBVersion),
			},
			Exporter: api.EtcdVersionExporter{
				Image: fmt.Sprintf("%s/operator:%s", DockerRegistry, ExporterTag),
			},
			Tools: api.EtcdVersionTools{
				Image: fmt.Sprintf("%s/etcd-tools:%s", DockerRegistry, DBVersion),
			},
		},
	}
}

func (f *Framework) CreateEtcdVersion(obj *api.EtcdVersion) error {
	_, err := f.extClient.CatalogV1alpha1().EtcdVersions().Create(obj)
	if err != nil && !kerr.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (f *Framework) DeleteEtcdVersion(meta metav1.ObjectMeta) error {
	return f.extClient.CatalogV1alpha1().EtcdVersions().Delete(meta.Name, &metav1.DeleteOptions{})
}
