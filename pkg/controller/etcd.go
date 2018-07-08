package controller

import (
	"fmt"

	"github.com/appscode/go/log"
	"github.com/appscode/kutil"
	meta_util "github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	"github.com/kubedb/apimachinery/pkg/eventer"
	"github.com/kubedb/apimachinery/pkg/storage"
	validator "github.com/kubedb/etcd/pkg/admission"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
)

func (c *Controller) create(etcd *api.Etcd) error {
	if err := validator.ValidateEtcd(c.Client, c.ExtClient, etcd); err != nil {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
			c.recorder.Event(
				ref,
				core.EventTypeWarning,
				eventer.EventReasonInvalid,
				err.Error())
		}
		log.Errorln(err)
		return nil
	}

	// Delete Matching DormantDatabase if exists any
	if err := c.deleteMatchingDormantDatabase(etcd); err != nil {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
			c.recorder.Eventf(
				ref,
				core.EventTypeWarning,
				eventer.EventReasonFailedToCreate,
				`Failed to delete dormant Database : "%v". Reason: %v`,
				etcd.Name,
				err,
			)
		}
		return err
	}

	if etcd.Status.CreationTime == nil {
		mg, _, err := util.PatchEtcd(c.ExtClient, etcd, func(in *api.Etcd) *api.Etcd {
			t := metav1.Now()
			in.Status.CreationTime = &t
			in.Status.Phase = api.DatabasePhaseCreating
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
			return err
		}
		etcd.Status = mg.Status
	}

	// create Governing Service
	governingService, err := c.createEtcdGoverningService(etcd)
	if err != nil {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
			c.recorder.Eventf(
				ref,
				core.EventTypeWarning,
				eventer.EventReasonFailedToCreate,
				`Failed to create Service: "%v". Reason: %v`,
				governingService,
				err,
			)
		}
		return err
	}
	fmt.Println(governingService, "-----------")
	c.GoverningService = governingService
	fmt.Println(c.GoverningService, ">>>>>>>>>>>>>>>>>>>")

	// ensure database Service
	/*vt1, err := c.ensureService(etcd)
	if err != nil {
		return err
	}*/

	// ensure database StatefulSet
	vt2, err := c.ensureEtcdNode(etcd)
	if err != nil {
		return err
	}

	if vt2 == kutil.VerbCreated {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
			c.recorder.Event(
				ref,
				core.EventTypeNormal,
				eventer.EventReasonSuccessful,
				"Successfully created Etcd",
			)
		}
	} else if vt2 == kutil.VerbPatched {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
			c.recorder.Event(
				ref,
				core.EventTypeNormal,
				eventer.EventReasonSuccessful,
				"Successfully patched Etcd",
			)
		}
	}

	if _, err := meta_util.GetString(etcd.Annotations, api.AnnotationInitialized); err == kutil.ErrNotFound &&
		etcd.Spec.Init != nil && etcd.Spec.Init.SnapshotSource != nil {

		snapshotSource := etcd.Spec.Init.SnapshotSource

		if etcd.Status.Phase == api.DatabasePhaseInitializing {
			return nil
		}
		jobName := fmt.Sprintf("%s-%s", api.DatabaseNamePrefix, snapshotSource.Name)
		if _, err := c.Client.BatchV1().Jobs(snapshotSource.Namespace).Get(jobName, metav1.GetOptions{}); err != nil {
			if kerr.IsAlreadyExists(err) {
				return nil
			} else if !kerr.IsNotFound(err) {
				return err
			}
		}
		if err := c.initialize(etcd); err != nil {
			return fmt.Errorf("failed to complete initialization. Reason: %v", err)
		}
		return nil
	}

	ms, _, err := util.PatchEtcd(c.ExtClient, etcd, func(in *api.Etcd) *api.Etcd {
		in.Status.Phase = api.DatabasePhaseRunning
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
		return err
	}
	etcd.Status = ms.Status

	// Ensure Schedule backup
	c.ensureBackupScheduler(etcd)

	if err := c.manageMonitor(etcd); err != nil {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
			c.recorder.Eventf(
				ref,
				core.EventTypeWarning,
				eventer.EventReasonFailedToCreate,
				"Failed to manage monitoring system. Reason: %v",
				err,
			)
		}
		log.Errorln(err)
		return nil
	}

	return nil
}

func (c *Controller) ensureEtcdNode(etcd *api.Etcd) (kutil.VerbType, error) {
	var err error

	if err := c.ensureDatabaseSecret(etcd); err != nil {
		return kutil.VerbUnchanged, err
	}
	if c.EnableRBAC {
		// Ensure ClusterRoles for database statefulsets
		if err := c.ensureRBACStuff(etcd); err != nil {
			return kutil.VerbUnchanged, err
		}
	}

	vt, err := c.ensureCombinedNode(etcd)
	if err != nil {
		return kutil.VerbUnchanged, err
	}

	return vt, nil
}

func (c *Controller) ensureBackupScheduler(etcd *api.Etcd) {
	// Setup Schedule backup
	if etcd.Spec.BackupSchedule != nil {
		err := c.cronController.ScheduleBackup(etcd, etcd.ObjectMeta, etcd.Spec.BackupSchedule)
		if err != nil {
			log.Errorln(err)
			if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
				c.recorder.Eventf(
					ref,
					core.EventTypeWarning,
					eventer.EventReasonFailedToSchedule,
					"Failed to schedule snapshot. Reason: %v",
					err,
				)
			}
		}
	} else {
		c.cronController.StopBackupScheduling(etcd.ObjectMeta)
	}
}

func (c *Controller) initialize(etcd *api.Etcd) error {
	mg, _, err := util.PatchEtcd(c.ExtClient, etcd, func(in *api.Etcd) *api.Etcd {
		in.Status.Phase = api.DatabasePhaseInitializing
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
		return err
	}
	etcd.Status = mg.Status

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

	namespace := snapshotSource.Namespace
	if namespace == "" {
		namespace = etcd.Namespace
	}
	snapshot, err := c.ExtClient.Snapshots(namespace).Get(snapshotSource.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	secret, err := storage.NewOSMSecret(c.Client, snapshot)
	if err != nil {
		return err
	}
	secret, err = c.Client.CoreV1().Secrets(secret.Namespace).Create(secret)
	if err != nil && !kerr.IsAlreadyExists(err) {
		return err
	}

	job, err := c.createRestoreJob(etcd, snapshot)
	if err != nil {
		return err
	}

	if err := c.SetJobOwnerReference(snapshot, job); err != nil {
		return err
	}

	return nil
}

func (c *Controller) pause(etcd *api.Etcd) error {
	if _, err := c.createDormantDatabase(etcd); err != nil {
		if kerr.IsAlreadyExists(err) {
			// if already exists, check if it is database of another Kind and return error in that case.
			// If the Kind is same, we can safely assume that the DormantDB was not deleted in before,
			// Probably because, User is more faster (create-delete-create-again-delete...) than operator!
			// So reuse that DormantDB!
			ddb, err := c.ExtClient.DormantDatabases(etcd.Namespace).Get(etcd.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if val, _ := meta_util.GetStringValue(ddb.Labels, api.LabelDatabaseKind); val != api.ResourceKindEtcd {
				return fmt.Errorf(`DormantDatabase "%v" of kind %v already exists`, etcd.Name, val)
			}
		} else {
			return fmt.Errorf(`Failed to create DormantDatabase: "%v". Reason: %v`, etcd.Name, err)
		}
	}

	c.cronController.StopBackupScheduling(etcd.ObjectMeta)

	if etcd.Spec.Monitor != nil {
		if _, err := c.deleteMonitor(etcd); err != nil {
			log.Errorln(err)
			return nil
		}
	}
	return nil
}
