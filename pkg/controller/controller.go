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
package controller

import (
	catalog "kubedb.dev/apimachinery/apis/catalog/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	cs "kubedb.dev/apimachinery/client/clientset/versioned"
	kutildb "kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	api_listers "kubedb.dev/apimachinery/client/listers/kubedb/v1alpha1"
	amc "kubedb.dev/apimachinery/pkg/controller"
	"kubedb.dev/apimachinery/pkg/controller/dormantdatabase"
	snapc "kubedb.dev/apimachinery/pkg/controller/snapshot"
	"kubedb.dev/apimachinery/pkg/eventer"

	"github.com/appscode/go/log"
	pcm "github.com/coreos/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	core "k8s.io/api/core/v1"
	crd_api "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	crd_cs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kwatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	reg_util "kmodules.xyz/client-go/admissionregistration/v1beta1"
	apiext_util "kmodules.xyz/client-go/apiextensions/v1beta1"
	"kmodules.xyz/client-go/tools/queue"
	appcat "kmodules.xyz/custom-resources/apis/appcatalog/v1alpha1"
	appcat_cs "kmodules.xyz/custom-resources/client/clientset/versioned"
)

type Event struct {
	Type   kwatch.EventType
	Object *api.Etcd
}

type Controller struct {
	amc.Config
	*amc.Controller

	// Prometheus client
	promClient pcm.MonitoringV1Interface
	// Cron Controller
	cronController snapc.CronControllerInterface
	// Event Recorder
	recorder record.EventRecorder
	// labelselector for event-handler of Snapshot, Dormant and Job
	selector labels.Selector

	clusters map[string]*Cluster
	// Etcd
	etcdQueue    *queue.Worker
	etcdInformer cache.SharedIndexInformer
	etcdLister   api_listers.EtcdLister
}

var _ amc.Snapshotter = &Controller{}
var _ amc.Deleter = &Controller{}

func New(
	clientConfig *rest.Config,
	client kubernetes.Interface,
	apiExtKubeClient crd_cs.ApiextensionsV1beta1Interface,
	extClient cs.Interface,
	dc dynamic.Interface,
	appCatalogClient appcat_cs.Interface,
	promClient pcm.MonitoringV1Interface,
	cronController snapc.CronControllerInterface,
	opt amc.Config,
	recorder record.EventRecorder,
) *Controller {
	return &Controller{
		Controller: &amc.Controller{
			ClientConfig:     clientConfig,
			Client:           client,
			ExtClient:        extClient,
			ApiExtKubeClient: apiExtKubeClient,
			DynamicClient:    dc,
			AppCatalogClient: appCatalogClient,
		},
		Config:         opt,
		promClient:     promClient,
		cronController: cronController,
		recorder:       recorder,
		selector: labels.SelectorFromSet(map[string]string{
			api.LabelDatabaseKind: api.ResourceKindEtcd,
		}),
		clusters: make(map[string]*Cluster),
	}
}

// EnsureCustomResourceDefinitions ensures CRD for MySQl, DormantDatabase and Snapshot
func (c *Controller) EnsureCustomResourceDefinitions() error {
	log.Infoln("Ensuring CustomResourceDefinition...")
	crds := []*crd_api.CustomResourceDefinition{
		api.Etcd{}.CustomResourceDefinition(),
		catalog.EtcdVersion{}.CustomResourceDefinition(),
		api.DormantDatabase{}.CustomResourceDefinition(),
		api.Snapshot{}.CustomResourceDefinition(),
		appcat.AppBinding{}.CustomResourceDefinition(),
	}
	return apiext_util.RegisterCRDs(c.ApiExtKubeClient, crds)
}

// InitInformer initializes Etcd, DormantDB amd Snapshot watcher
func (c *Controller) Init() error {
	c.initWatcher()
	c.DrmnQueue = dormantdatabase.NewController(c.Controller, c, c.Config, nil, c.recorder).AddEventHandlerFunc(c.selector)
	c.SnapQueue, c.JobQueue = snapc.NewController(c.Controller, c, c.Config, nil, c.recorder).AddEventHandlerFunc(c.selector)

	return nil
}

// RunControllers runs queue.worker
func (c *Controller) RunControllers(stopCh <-chan struct{}) {
	// Start Cron
	c.cronController.StartCron()

	// Watch x  TPR objects
	c.etcdQueue.Run(stopCh)
	c.DrmnQueue.Run(stopCh)
	c.SnapQueue.Run(stopCh)
	c.JobQueue.Run(stopCh)
}

// Blocks caller. Intended to be called as a Go routine.
func (c *Controller) Run(stopCh <-chan struct{}) {
	go c.StartAndRunControllers(stopCh)

	if c.EnableMutatingWebhook {
		cancel1, _ := reg_util.SyncMutatingWebhookCABundle(c.ClientConfig, mutatingWebhookConfig)
		defer cancel1()
	}
	if c.EnableValidatingWebhook {
		cancel2, _ := reg_util.SyncValidatingWebhookCABundle(c.ClientConfig, validatingWebhookConfig)
		defer cancel2()
	}

	<-stopCh
	c.cronController.StopCron()
}

// StartAndRunControllers starts InformetFactory and runs queue.worker
func (c *Controller) StartAndRunControllers(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	log.Infoln("Starting KubeDB controller")
	c.KubeInformerFactory.Start(stopCh)
	c.KubedbInformerFactory.Start(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	for t, v := range c.KubeInformerFactory.WaitForCacheSync(stopCh) {
		if !v {
			log.Fatalf("%v timed out waiting for caches to sync", t)
			return
		}
	}
	for t, v := range c.KubedbInformerFactory.WaitForCacheSync(stopCh) {
		if !v {
			log.Fatalf("%v timed out waiting for caches to sync", t)
			return
		}
	}

	c.RunControllers(stopCh)

	<-stopCh
	log.Infoln("Stopping KubeDB controller")
}

func (c *Controller) pushFailureEvent(etcd *api.Etcd, reason string) {
	if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
		c.recorder.Eventf(
			ref,
			core.EventTypeWarning,
			eventer.EventReasonFailedToStart,
			`Fail to be ready Etcd: "%v". Reason: %v`,
			etcd.Name,
			reason,
		)
	}

	db, err := kutildb.UpdateEtcdStatus(c.ExtClient.KubedbV1alpha1(), etcd, func(in *api.EtcdStatus) *api.EtcdStatus {
		in.Phase = api.DatabasePhaseFailed
		in.Reason = reason
		in.ObservedGeneration = etcd.Generation
		return in
	})
	if err != nil {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
			c.recorder.Eventf(
				ref,
				core.EventTypeWarning,
				eventer.EventReasonFailedToUpdate,
				err.Error(),
			)
		}
	}
	etcd.Status = db.Status
}
