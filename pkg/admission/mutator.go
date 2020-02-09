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
package admission

import (
	"fmt"
	"sync"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	cs "kubedb.dev/apimachinery/client/clientset/versioned"

	"github.com/appscode/go/log"
	"github.com/appscode/go/types"
	"github.com/pkg/errors"
	admission "k8s.io/api/admission/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kutil "kmodules.xyz/client-go"
	core_util "kmodules.xyz/client-go/core/v1"
	meta_util "kmodules.xyz/client-go/meta"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
	hookapi "kmodules.xyz/webhook-runtime/admission/v1beta1"
)

type EtcdMutator struct {
	client      kubernetes.Interface
	extClient   cs.Interface
	lock        sync.RWMutex
	initialized bool
}

var _ hookapi.AdmissionHook = &EtcdMutator{}

func (a *EtcdMutator) Resource() (plural schema.GroupVersionResource, singular string) {
	return schema.GroupVersionResource{
			Group:    "mutators.kubedb.com",
			Version:  "v1alpha1",
			Resource: "etcdmutators",
		},
		"etcdmutator"
}

func (a *EtcdMutator) Initialize(config *rest.Config, stopCh <-chan struct{}) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.initialized = true

	var err error
	if a.client, err = kubernetes.NewForConfig(config); err != nil {
		return err
	}
	if a.extClient, err = cs.NewForConfig(config); err != nil {
		return err
	}
	return err
}

func (a *EtcdMutator) Admit(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	status := &admission.AdmissionResponse{}

	// N.B.: No Mutating for delete
	if (req.Operation != admission.Create && req.Operation != admission.Update) ||
		len(req.SubResource) != 0 ||
		req.Kind.Group != api.SchemeGroupVersion.Group ||
		req.Kind.Kind != api.ResourceKindEtcd {
		status.Allowed = true
		return status
	}

	a.lock.RLock()
	defer a.lock.RUnlock()
	if !a.initialized {
		return hookapi.StatusUninitialized()
	}
	obj, err := meta_util.UnmarshalFromJSON(req.Object.Raw, api.SchemeGroupVersion)
	if err != nil {
		return hookapi.StatusBadRequest(err)
	}
	etcdMod, err := setDefaultValues(obj.(*api.Etcd).DeepCopy())
	if err != nil {
		return hookapi.StatusForbidden(err)
	} else if etcdMod != nil {
		patch, err := meta_util.CreateJSONPatch(req.Object.Raw, etcdMod)
		if err != nil {
			return hookapi.StatusInternalServerError(err)
		}
		status.Patch = patch
		patchType := admission.PatchTypeJSONPatch
		status.PatchType = &patchType
	}

	status.Allowed = true
	return status
}

// setDefaultValues provides the defaulting that is performed in mutating stage of creating/updating a Etcd database
func setDefaultValues(extClient cs.Interface, px *api.Etcd) (runtime.Object, error) {
	if px.Spec.Version == "" {
		return nil, errors.New(`'spec.version' is missing`)
	}

	if px.Spec.Halted {
		if px.Spec.TerminationPolicy == api.TerminationPolicyDoNotTerminate {
			return nil, errors.New(`Can't halt, since termination policy is 'DoNotTerminate'`)
		}
		px.Spec.TerminationPolicy = api.TerminationPolicyHalt
	}

	px.SetDefaults()

	// If monitoring spec is given without port,
	// set default Listening port
	setMonitoringPort(px)

	return px, nil
}

// Assign Default Monitoring Port if MonitoringSpec Exists
// and the AgentVendor is Prometheus.
func setMonitoringPort(etcd *api.Etcd) {
	if etcd.Spec.Monitor != nil &&
		etcd.GetMonitoringVendor() == mona.VendorPrometheus {
		if etcd.Spec.Monitor.Prometheus == nil {
			etcd.Spec.Monitor.Prometheus = &mona.PrometheusSpec{}
		}
		if etcd.Spec.Monitor.Prometheus.Exporter == nil {
			etcd.Spec.Monitor.Prometheus.Exporter = &mona.PrometheusExporterSpec{}
		}
		if etcd.Spec.Monitor.Prometheus.Exporter.Port == 0 {
			etcd.Spec.Monitor.Prometheus.Exporter.Port = 2379 //api.PrometheusExporterPortNumber
		}
	}
}
