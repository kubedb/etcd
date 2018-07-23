package cluster

import (
	"crypto/tls"
	"fmt"
	"reflect"
	"time"

	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	cs "github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1"
	dbutil "github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	"github.com/kubedb/etcd/pkg/docker"
	"github.com/kubedb/etcd/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

var (
	reconcileInterval         = 8 * time.Second
	podTerminationGracePeriod = int64(5)
)

type clusterEventType string

const (
	eventModifyCluster clusterEventType = "Modify"
)

type clusterEvent struct {
	typ     clusterEventType
	cluster *api.Etcd
}

type Config struct {
	ServiceAccount string

	docker    docker.Docker
	KubeCli   kubernetes.Interface
	EtcdCRCli cs.KubedbV1alpha1Interface
}

type Cluster struct {
	logger *logrus.Entry
	config Config

	cluster *api.Etcd

	// in memory state of the cluster
	// status is the source of truth after Cluster struct is materialized.
	status api.EtcdStatus

	eventCh chan *clusterEvent
	stopCh  chan struct{}

	// members repsersents the members in the etcd cluster.
	// the name of the member is the the name of the pod the member
	// process runs in.
	members util.MemberSet

	tlsConfig *tls.Config

	eventsCli corev1.EventInterface
}

func New(config Config, etcd *api.Etcd) *Cluster {
	lg := logrus.WithField("pkg", "cluster").WithField("cluster-name", etcd.Name)

	c := &Cluster{
		logger:    lg,
		config:    config,
		cluster:   etcd,
		eventCh:   make(chan *clusterEvent, 100),
		stopCh:    make(chan struct{}),
		status:    *(etcd.Status.DeepCopy()),
		eventsCli: config.KubeCli.Core().Events(etcd.Namespace),
	}
	fmt.Println("......................................")

	go func() {
		if err := c.setup(); err != nil {
			fmt.Println(err,"...........................")
			if c.status.Phase != api.DatabasePhaseFailed {
				c.status.Reason = err.Error()
				c.status.Phase = api.DatabasePhaseFailed
				if err := c.updateCRStatus(); err != nil {
					c.logger.Errorf("failed to update cluster phase (%v): %v", api.DatabasePhaseFailed, err)
				}
			}
			return
		}
		c.run()
	}()

	return c
}

func (c *Cluster) setup() error {
	var shouldCreateCluster bool
	switch c.status.Phase {
	case "":
		shouldCreateCluster = true
	case api.DatabasePhaseCreating:
		return errors.New("cluster failed to be created")
	case api.DatabasePhaseRunning:
		shouldCreateCluster = false
	default:
		return fmt.Errorf("unexpected cluster phase: %s", c.status.Phase)
	}

	/*if c.isSecureClient() {
		d, err := k8sutil.GetTLSDataFromSecret(c.config.KubeCli, c.cluster.Namespace, c.cluster.Spec.TLS.Static.OperatorSecret)
		if err != nil {
			return err
		}
		c.tlsConfig, err = etcdutil.NewTLSConfig(d.CertData, d.KeyData, d.CAData)
		if err != nil {
			return err
		}
	}*/

	if shouldCreateCluster {
		return c.create()
	}
	return nil
}

func (c *Cluster) create() error {
	c.status.Phase = api.DatabasePhaseCreating

	fmt.Println("creating..................")
	if err := c.updateCRStatus(); err != nil {
		return fmt.Errorf("cluster create: failed to update cluster phase (%v): %v", api.DatabasePhaseCreating, err)
	}

	return c.prepareSeedMember()
}

func (c *Cluster) prepareSeedMember() error {

	err := c.bootstrap()
	if err != nil {
		return err
	}

	return nil
}

func (c *Cluster) bootstrap() error {
	return c.startSeedMember()
}

func (c *Cluster) Update(cl *api.Etcd) {
	c.send(&clusterEvent{
		typ:     eventModifyCluster,
		cluster: cl,
	})
}

func (c *Cluster) Delete() {
	c.logger.Info("cluster is deleted by user")
	close(c.stopCh)
}

func (c *Cluster) startSeedMember() error {
	m := &util.Member{
		Name:         util.UniqueMemberName(c.cluster.Name),
		Namespace:    c.cluster.Namespace,
		SecurePeer:   c.isSecurePeer(),
		SecureClient: c.isSecureClient(),
	}
	ms := util.NewMemberSet(m)
	if _, _, err := c.createPod(ms, m, "new"); err != nil {
		return fmt.Errorf("failed to create seed member (%s): %v", m.Name, err)
	}
	c.members = ms
	c.logger.Infof("cluster created with seed member (%s)", m.Name)
	_, err := c.eventsCli.Create(util.NewMemberAddEvent(m.Name, c.cluster))
	if err != nil {
		c.logger.Errorf("failed to create new member add event: %v", err)
	}

	return nil
}

func (c *Cluster) run() {
	c.status.Phase = api.DatabasePhaseCreating
	if err := c.updateCRStatus(); err != nil {
		fmt.Println("W: update initial CR status failed: %v", err)
	}
	fmt.Println("start running...")

	var rerr error
	for {
		select {
		case <-c.stopCh:
			return
		case event := <-c.eventCh:
			switch event.typ {
			case eventModifyCluster:
				err := c.handleUpdateEvent(event)
				if err != nil {
					c.status.Reason = err.Error()
					//c.reportFailedStatus()
					return
				}
			default:
				panic("unknown event type" + event.typ)
			}

		case <-time.After(reconcileInterval):
			//start := time.Now()

			if !c.cluster.Spec.DoNotPause {
				//c.status.PauseControl()
				c.logger.Infof("control is paused, skipping reconciliation")
				continue
			} else {
				//	c.status.Control()
			}

			running, pending, err := c.pollPods()
			if err != nil {
				//	c.logger.Errorf("fail to poll pods: %v", err)
				//	reconcileFailed.WithLabelValues("failed to poll pods").Inc()
				continue
			}

			if len(pending) > 0 {
				// Pod startup might take long, e.g. pulling image. It would deterministically become running or succeeded/failed later.
				c.logger.Infof("skip reconciliation: running (%v), pending (%v)", running, pending)
				//	reconcileFailed.WithLabelValues("not all pods are running").Inc()
				continue
			}
			if len(running) == 0 {
				// TODO: how to handle this case?
				c.logger.Warningf("all etcd pods are dead.")
				break
			}

			// On controller restore, we could have "members == nil"
			if rerr != nil || c.members == nil {
				rerr = c.updateMembers(podsToMemberSet(running, c.isSecureClient()))
				if rerr != nil {
					c.logger.Errorf("failed to update members: %v", rerr)
					break
				}
			}
			rerr = c.reconcile(running)
			if rerr != nil {
				c.logger.Errorf("failed to reconcile: %v", rerr)
				break
			}
			//c.updateMemberStatus(running)
			if err := c.updateCRStatus(); err != nil {
				c.logger.Warningf("periodic update CR status failed: %v", err)
			}

			//reconcileHistogram.WithLabelValues(c.name()).Observe(time.Since(start).Seconds())
		}

		if rerr != nil {
			//reconcileFailed.WithLabelValues(rerr.Error()).Inc()
		}

		/*if isFatalError(rerr) {
			c.status.SetReason(rerr.Error())
			c.logger.Errorf("cluster failed: %v", rerr)
			c.reportFailedStatus()
			return
		}*/
	}
}

func (c *Cluster) handleUpdateEvent(event *clusterEvent) error {
	//	oldSpec := c.cluster.Spec.DeepCopy()
	c.cluster = event.cluster

	/*	if isSpecEqual(event.cluster.Spec, *oldSpec) {
			// We have some fields that once created could not be mutated.
			if !reflect.DeepEqual(event.cluster.Spec, *oldSpec) {
				c.logger.Infof("ignoring update event: %#v", event.cluster.Spec)
			}
			return nil
		}
		// TODO: we can't handle another upgrade while an upgrade is in progress

		c.logSpecUpdate(*oldSpec, event.cluster.Spec)*/
	return nil
}

func (c *Cluster) pollPods() (running, pending []*v1.Pod, err error) {
	podList, err := c.config.KubeCli.Core().Pods(c.cluster.Namespace).List(metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"etcd_cluster": c.cluster.Name,
			"app":          "etcd",
		}).String(),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list running pods: %v", err)
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		// Avoid polling deleted pods. k8s issue where deleted pods would sometimes show the status Pending
		// See https://github.com/coreos/etcd-operator/issues/1693
		if pod.DeletionTimestamp != nil {
			continue
		}
		if len(pod.OwnerReferences) < 1 {
			fmt.Println("pollPods: ignore pod %v: no owner", pod.Name)
			continue
		}
		if pod.OwnerReferences[0].UID != c.cluster.UID {
			fmt.Println("pollPods: ignore pod %v: owner (%v) is not %v",
				pod.Name, pod.OwnerReferences[0].UID, c.cluster.UID)
			continue
		}
		switch pod.Status.Phase {
		case v1.PodRunning:
			running = append(running, pod)
		case v1.PodPending:
			pending = append(pending, pod)
		}
	}

	return running, pending, nil
}

func (c *Cluster) removePod(name string) error {
	ns := c.cluster.Namespace
	opts := metav1.NewDeleteOptions(podTerminationGracePeriod)
	err := c.config.KubeCli.Core().Pods(ns).Delete(name, opts)
	if err != nil {
		/*if !util.IsKubernetesResourceNotFoundError(err) {
			return err
		}*/
	}
	return nil
}

func (c *Cluster) send(ev *clusterEvent) {
	select {
	case c.eventCh <- ev:
		l, ecap := len(c.eventCh), cap(c.eventCh)
		if l > int(float64(ecap)*0.8) {
			c.logger.Warningf("eventCh buffer is almost full [%d/%d]", l, ecap)
		}
	case <-c.stopCh:
	}
}

func (c *Cluster) isSecurePeer() bool {
	if c.cluster.Spec.TLS == nil || c.cluster.Spec.TLS.Member == nil {
		return false
	}
	return len(c.cluster.Spec.TLS.Member.PeerSecret) != 0
}

func (c *Cluster) isSecureClient() bool {
	if c.cluster.Spec.TLS == nil {
		return false
	}
	return len(c.cluster.Spec.TLS.OperatorSecret) != 0
}

func (c *Cluster) updateCRStatus() error {
	if reflect.DeepEqual(c.cluster.Status, c.status) {
		return nil
	}
	_, _, err := dbutil.PatchEtcd(c.config.EtcdCRCli, c.cluster, func(in *api.Etcd) *api.Etcd {

		in.Status.Phase = c.status.Phase
		return in
	})
	return err
}
