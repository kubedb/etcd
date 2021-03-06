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
	"context"
	"fmt"
	"strings"
	"time"

	"kubedb.dev/etcd/pkg/controller"

	goetcd "github.com/coreos/etcd/client"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kmodules.xyz/client-go/tools/portforward"
)

type KubedbTable struct {
	Key   string
	Value string
}

const (
	EtcdTestKey   = "testKey"
	EtcdTestValue = "testValue"
)

func (f *Framework) ForwardPort(meta metav1.ObjectMeta) (*portforward.Tunnel, error) {
	clientPodName, err := f.GetEtcdClientPod(meta)
	if err != nil {
		return nil, err
	}

	tunnel := portforward.NewTunnel(
		f.kubeClient.CoreV1().RESTClient(),
		f.restConfig,
		meta.Namespace,
		clientPodName,
		controller.EtcdClientPort,
	)
	if err := tunnel.ForwardPort(); err != nil {
		return nil, err
	}

	return tunnel, nil
}

func (f *Framework) GetEtcdClient(tunnel *portforward.Tunnel) (goetcd.Client, error) {
	cfg := goetcd.Config{
		Endpoints:               []string{fmt.Sprintf("http://127.0.0.1:%v", tunnel.Local)},
		Transport:               goetcd.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
	}
	return goetcd.New(cfg)
}

func (f *Framework) GetEtcdClientPod(meta metav1.ObjectMeta) (string, error) {
	pods, err := f.kubeClient.CoreV1().Pods(meta.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, meta.Name) {
			return pod.Name, nil
		}
	}
	return "", fmt.Errorf("no client pod found for %s", meta.Name)
}

func (f *Framework) EventuallyDatabaseReady(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			tunnel, err := f.ForwardPort(meta)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			client, err := f.GetEtcdClient(tunnel)
			if err != nil {
				return false
			}
			kapi := goetcd.NewKeysAPI(client)
			_, err = kapi.Set(context.Background(), "/foo", "bar", nil)
			if err != nil {
				return false
			}
			_, err = kapi.Delete(context.Background(), "/foo", nil)
			if err != nil {
				return false
			}

			return true
		},
		time.Minute*15,
		time.Second*10,
	)
}

func (f *Framework) EventuallySetKey(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			tunnel, err := f.ForwardPort(meta)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			client, err := f.GetEtcdClient(tunnel)
			if err != nil {
				return false
			}
			kapi := goetcd.NewKeysAPI(client)
			_, err = kapi.Set(context.Background(), EtcdTestKey, EtcdTestValue, nil)
			if err != nil {
				return false
			}
			_, err = kapi.Get(context.Background(), EtcdTestKey, nil)
			if err != nil {
				return false
			}
			return true
		},
		time.Minute*15,
		time.Second*10,
	)
}

func (f *Framework) EventuallyKeyExists(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			tunnel, err := f.ForwardPort(meta)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			client, err := f.GetEtcdClient(tunnel)
			if err != nil {
				return false
			}
			kapi := goetcd.NewKeysAPI(client)
			resp, err := kapi.Get(context.Background(), EtcdTestKey, nil)
			if err != nil {
				return false
			}
			if resp.Node.Value == EtcdTestValue {
				return true
			}

			return false
		},
		time.Minute*15,
		time.Second*10,
	)
}
