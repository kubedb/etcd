package cluster

import (
	"context"
	"errors"
	"fmt"

	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/coreos/etcd-operator/pkg/util/constants"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/coreos/etcd-operator/pkg/util/k8sutil"

	"github.com/coreos/etcd/clientv3"
	"github.com/pborman/uuid"
	"k8s.io/api/core/v1"
	"log"
	"github.com/kubedb/etcd/pkg/util"
	"github.com/appscode/kutil"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
)

var ErrLostQuorum = errors.New("lost quorum")


func (c *Cluster) reconcile(pods []*v1.Pod) error {
	sp := c.cluster.Spec
	running := podsToMemberSet(pods, c.isSecureClient())
	if !running.IsEqual(c.members) || int32(c.members.Size()) != *sp.Replicas {
		return c.reconcileMembers(running)
	}
	//c.status.ClearCondition(api.ClusterConditionScaling)


	if needUpgrade(pods, sp) {
		c.status.UpgradeVersionTo(sp.Version)

		m := pickOneOldMember(pods, string(sp.Version))
		return c.upgradeOneMember(m.Name)
	}
	c.status.ClearCondition(api.ClusterConditionUpgrading)

	c.status.SetVersion(sp.Version)
	c.status.SetReadyCondition()

	return nil
}

func (c *Cluster) reconcileMembers(running util.MemberSet) error {
	log.Println("running members: %s", running)
	log.Println("cluster membership: %s", c.members)

	unknownMembers := running.Diff(c.members)
	if unknownMembers.Size() > 0 {
		log.Println("removing unexpected pods: %v", unknownMembers)
		for _, m := range unknownMembers {
			if err := c.removePod(m.Name); err != nil {
				return err
			}
		}
	}
	L := running.Diff(unknownMembers)

	if L.Size() == c.members.Size() {
		return c.resize()
	}

	if L.Size() < c.members.Size()/2+1 {
		return ErrLostQuorum
	}

	log.Println("removing one dead member")
	// remove dead members that doesn't have any running pods before doing resizing.
	return c.removeDeadMember(c.members.Diff(L).PickOne())
}

func (c *Cluster) resize() error {
	if c.members.Size() == int(*c.cluster.Spec.Replicas) {
		return nil
	}

	if c.members.Size() < int(*c.cluster.Spec.Replicas) {
		return c.addOneMember()
	}

	return c.removeOneMember()
}

func (c *Cluster) addOneMember() error {
	cfg := clientv3.Config{
		Endpoints:   c.members.ClientURLs(),
		DialTimeout: constants.DefaultDialTimeout,
		TLS:         c.tlsConfig,
	}
	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		return fmt.Errorf("add one member failed: creating etcd client failed %v", err)
	}
	defer etcdcli.Close()

	newMember := c.newMember()
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	resp, err := etcdcli.MemberAdd(ctx, []string{newMember.PeerURL()})
	cancel()
	if err != nil {
		return fmt.Errorf("fail to add new member (%s): %v", newMember.Name, err)
	}
	newMember.ID = resp.Member.ID
	c.members.Add(newMember)

	_, _, err = c.createPod(newMember, c.members.PeerURLPairs(), c.cluster.Name,  "existing", uuid.New())
	if err != nil {
		return fmt.Errorf("fail to create member's pod (%s): %v", newMember.Name, err)
	}
	log.Println("added member (%s)", newMember.Name)
	// Check StatefulSet Pod status
	/*if vt != kutil.VerbUnchanged {
		if err := c.checkStatefulSetPodStatus(statefulSet); err != nil {
			if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
				c.recorder.Eventf(
					ref,
					v1.EventTypeWarning,
					eventer.EventReasonFailedToStart,
					`Failed to CreateOrPatch StatefulSet. Reason: %v`,
					err,
				)
			}
			return kutil.VerbUnchanged, err
		}
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
			c.recorder.Eventf(
				ref,
				v1.EventTypeNormal,
				eventer.EventReasonSuccessful,
				"Successfully %v StatefulSet",
				vt,
			)
		}
	}*/
	return nil
}

func (c *Cluster) removeOneMember() error {
	return c.removeMember(c.members.PickOne())
}

func (c *Cluster) removeDeadMember(toRemove *util.Member) error {
	log.Println("removing dead member %q", toRemove.Name)
	/*_, err := c.eventsCli.Create(k8sutil.ReplacingDeadMemberEvent(toRemove.Name, c.cluster))
	if err != nil {
		c.logger.Errorf("failed to create replacing dead member event: %v", err)
	}*/

	return c.removeMember(toRemove)
}

func (c *Cluster) removeMember(toRemove *util.Member) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("remove member (%s) failed: %v", toRemove.Name, err)
		}
	}()

	err = etcdutil.RemoveMember(c.members.ClientURLs(), c.tlsConfig, toRemove.ID)
	if err != nil {
		switch err {
		case rpctypes.ErrMemberNotFound:
			log.Println("etcd member (%v) has been removed", toRemove.Name)
		default:
			return err
		}
	}
	c.members.Remove(toRemove.Name)

	if err := c.removePod(toRemove.Name); err != nil {
		return err
	}
	/*if c.isPodPVEnabled() {
		err = c.removePVC(k8sutil.PVCNameFromMember(toRemove.Name))
		if err != nil {
			return err
		}
	}
	c.logger.Infof("removed member (%v) with ID (%d)", toRemove.Name, toRemove.ID)*/
	return nil
}

func needUpgrade(pods []*v1.Pod, cs api.EtcdSpec) bool {
	return len(pods) == int(*cs.Replicas) && pickOneOldMember(pods, string(cs.Version)) != nil
}

func pickOneOldMember(pods []*v1.Pod, newVersion string) *util.Member {
	for _, pod := range pods {
		if util.GetEtcdVersion(pod) == newVersion {
			continue
		}
		return &util.Member{Name: pod.Name, Namespace: pod.Namespace}
	}
	return nil
}
