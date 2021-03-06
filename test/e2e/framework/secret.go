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
	"io/ioutil"
	"os"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/etcd/pkg/controller"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/log"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	store "kmodules.xyz/objectstore-api/api/v1"
)

func (fi *Invocation) SecretForLocalBackend() *core.Secret {
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(fi.app + "-local"),
			Namespace: fi.namespace,
		},
		Data: map[string][]byte{},
	}
}

func (fi *Invocation) SecretForS3Backend() *core.Secret {
	if os.Getenv(store.AWS_ACCESS_KEY_ID) == "" ||
		os.Getenv(store.AWS_SECRET_ACCESS_KEY) == "" {
		return &core.Secret{}
	}

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(fi.app + "-s3"),
			Namespace: fi.namespace,
		},
		Data: map[string][]byte{
			store.AWS_ACCESS_KEY_ID:     []byte(os.Getenv(store.AWS_ACCESS_KEY_ID)),
			store.AWS_SECRET_ACCESS_KEY: []byte(os.Getenv(store.AWS_SECRET_ACCESS_KEY)),
		},
	}
}

func (fi *Invocation) SecretForGCSBackend() *core.Secret {
	if os.Getenv(store.GOOGLE_PROJECT_ID) == "" ||
		(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" && os.Getenv(store.GOOGLE_SERVICE_ACCOUNT_JSON_KEY) == "") {
		return &core.Secret{}
	}

	jsonKey := os.Getenv(store.GOOGLE_SERVICE_ACCOUNT_JSON_KEY)
	if jsonKey == "" {
		if keyBytes, err := ioutil.ReadFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")); err == nil {
			jsonKey = string(keyBytes)
		}
	}

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(fi.app + "-gcs"),
			Namespace: fi.namespace,
		},
		Data: map[string][]byte{
			store.GOOGLE_PROJECT_ID:               []byte(os.Getenv(store.GOOGLE_PROJECT_ID)),
			store.GOOGLE_SERVICE_ACCOUNT_JSON_KEY: []byte(jsonKey),
		},
	}
}

func (fi *Invocation) SecretForAzureBackend() *core.Secret {
	if os.Getenv(store.AZURE_ACCOUNT_NAME) == "" ||
		os.Getenv(store.AZURE_ACCOUNT_KEY) == "" {
		return &core.Secret{}
	}

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(fi.app + "-azure"),
			Namespace: fi.namespace,
		},
		Data: map[string][]byte{
			store.AZURE_ACCOUNT_NAME: []byte(os.Getenv(store.AZURE_ACCOUNT_NAME)),
			store.AZURE_ACCOUNT_KEY:  []byte(os.Getenv(store.AZURE_ACCOUNT_KEY)),
		},
	}
}

func (fi *Invocation) SecretForSwiftBackend() *core.Secret {
	if os.Getenv(store.OS_AUTH_URL) == "" ||
		(os.Getenv(store.OS_TENANT_ID) == "" && os.Getenv(store.OS_TENANT_NAME) == "") ||
		os.Getenv(store.OS_USERNAME) == "" ||
		os.Getenv(store.OS_PASSWORD) == "" {
		return &core.Secret{}
	}

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(fi.app + "-swift"),
			Namespace: fi.namespace,
		},
		Data: map[string][]byte{
			store.OS_AUTH_URL:    []byte(os.Getenv(store.OS_AUTH_URL)),
			store.OS_TENANT_ID:   []byte(os.Getenv(store.OS_TENANT_ID)),
			store.OS_TENANT_NAME: []byte(os.Getenv(store.OS_TENANT_NAME)),
			store.OS_USERNAME:    []byte(os.Getenv(store.OS_USERNAME)),
			store.OS_PASSWORD:    []byte(os.Getenv(store.OS_PASSWORD)),
			store.OS_REGION_NAME: []byte(os.Getenv(store.OS_REGION_NAME)),
		},
	}
}

func (f *Framework) CreateSecret(obj *core.Secret) error {
	_, err := f.kubeClient.CoreV1().Secrets(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) UpdateSecret(meta metav1.ObjectMeta, transformer func(core.Secret) core.Secret) error {
	attempt := 0
	for ; attempt < maxAttempts; attempt = attempt + 1 {
		cur, err := f.kubeClient.CoreV1().Secrets(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(err) {
			return nil
		} else if err == nil {
			modified := transformer(*cur)
			_, err = f.kubeClient.CoreV1().Secrets(cur.Namespace).Update(&modified)
			if err == nil {
				return nil
			}
		}
		log.Errorf("Attempt %d failed to update Secret %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(updateRetryInterval)
	}
	return fmt.Errorf("failed to update Secret %s@%s after %d attempts", meta.Name, meta.Namespace, attempt)
}

func (f *Framework) GetEtcdRootPassword(etcd *api.Etcd) (string, error) {
	secret, err := f.kubeClient.CoreV1().Secrets(etcd.Namespace).Get(etcd.Spec.DatabaseSecret.SecretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	password := string(secret.Data[controller.KeyEtcdPassword])
	return password, nil
}

func (f *Framework) DeleteSecret(meta metav1.ObjectMeta) error {
	return f.kubeClient.CoreV1().Secrets(meta.Namespace).Delete(meta.Name, deleteInBackground())
}
