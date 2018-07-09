package controller

import (
	"github.com/appscode/go/log"
	core_util "github.com/appscode/kutil/core/v1"
	meta_util "github.com/appscode/kutil/meta"
	"github.com/appscode/kutil/tools/queue"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
)

func (c *Controller) initWatcher() {
	c.etcdInformer = c.KubedbInformerFactory.Kubedb().V1alpha1().Etcds().Informer()
	c.etcdQueue = queue.New("Etcd", c.MaxNumRequeues, c.NumThreads, c.runEtcd)
	c.etcdLister = c.KubedbInformerFactory.Kubedb().V1alpha1().Etcds().Lister()
	c.etcdInformer.AddEventHandler(queue.NewEventHandler(c.etcdQueue.GetQueue(), func(old interface{}, new interface{}) bool {
		oldObj := old.(*api.Etcd)
		newObj := new.(*api.Etcd)
		return newObj.DeletionTimestamp != nil || !etcdEqual(oldObj, newObj)
	}))
}

func etcdEqual(old, new *api.Etcd) bool {
	if !meta_util.Equal(old.Spec, new.Spec) {
		diff := meta_util.Diff(old.Spec, new.Spec)
		log.Infof("Etcd %s/%s has changed. Diff: %s", new.Namespace, new.Name, diff)
		return false
	}
	if !meta_util.Equal(old.Annotations, new.Annotations) {
		diff := meta_util.Diff(old.Annotations, new.Annotations)
		log.Infof("Annotations in Etcd %s/%s has changed. Diff: %s\n", new.Namespace, new.Name, diff)
		return false
	}
	return true
}

func (c *Controller) runEtcd(key string) error {
	log.Debugln("started processing, key:", key)
	obj, exists, err := c.etcdInformer.GetIndexer().GetByKey(key)
	if err != nil {
		log.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	if !exists {
		log.Debugf("Etcd %s does not exist anymore\n", key)
	} else {
		// Note that you also have to check the uid if you have a local controlled resource, which
		// is dependent on the actual instance, to detect that a Etcd was recreated with the same name
		etcd := obj.(*api.Etcd).DeepCopy()
		if etcd.DeletionTimestamp != nil {
			if core_util.HasFinalizer(etcd.ObjectMeta, api.GenericKey) {
				util.AssignTypeKind(etcd)
				if err := c.pause(etcd); err != nil {
					log.Errorln(err)
					return err
				}
				etcd, _, err = util.PatchEtcd(c.ExtClient, etcd, func(in *api.Etcd) *api.Etcd {
					in.ObjectMeta = core_util.RemoveFinalizer(in.ObjectMeta, api.GenericKey)
					return in
				})
				return err
			}
		} else {
			etcd, _, err = util.PatchEtcd(c.ExtClient, etcd, func(in *api.Etcd) *api.Etcd {
				in.ObjectMeta = core_util.AddFinalizer(in.ObjectMeta, api.GenericKey)
				return in
			})
			util.AssignTypeKind(etcd)
			if err := c.syncEtcd(etcd); err != nil {
				log.Errorln(err)
				c.pushFailureEvent(etcd, err.Error())
				return err
			}
		}
	}
	return nil
}
