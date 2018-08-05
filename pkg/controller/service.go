package controller

import (
	core_util "github.com/appscode/kutil/core/v1"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"k8s.io/api/core/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
)

const TolerateUnreadyEndpointsAnnotation = "service.alpha.kubernetes.io/tolerate-unready-endpoints"

func (c *Controller) CreateClientService(cl *Cluster) error {
	ports := []v1.ServicePort{{
		Name:       "client",
		Port:       EtcdClientPort,
		TargetPort: intstr.FromInt(EtcdClientPort),
		Protocol:   v1.ProtocolTCP,
	}}

	return createService(c.Controller.Client, ClientServiceName(cl.cluster.Name), cl.cluster.Namespace, "", ports, cl.cluster)
}

func ClientServiceName(clusterName string) string {
	return clusterName + "-client"
}

func (c *Controller) CreatePeerService(cl *Cluster) error {
	ports := []v1.ServicePort{{
		Name:       "client",
		Port:       EtcdClientPort,
		TargetPort: intstr.FromInt(EtcdClientPort),
		Protocol:   v1.ProtocolTCP,
	}, {
		Name:       "peer",
		Port:       2380,
		TargetPort: intstr.FromInt(2380),
		Protocol:   v1.ProtocolTCP,
	}}

	return createService(c.Controller.Client, cl.cluster.Name, cl.cluster.Namespace, v1.ClusterIPNone, ports, cl.cluster)
}

func createService(kubecli kubernetes.Interface, svcName, ns, clusterIP string, ports []v1.ServicePort, etcd *api.Etcd) error {
	meta := metav1.ObjectMeta{
		Name:      svcName,
		Namespace: etcd.Namespace,
	}

	ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd)
	if rerr != nil {
		return rerr
	}

	_, _, err := core_util.CreateOrPatchService(kubecli, meta, func(in *core.Service) *core.Service {
		in.ObjectMeta = core_util.EnsureOwnerReference(in.ObjectMeta, ref)
		in.Labels = etcd.OffshootLabels()
		in.Annotations = etcd.Spec.ServiceTemplate.Annotations
		if in.Annotations == nil {
			in.Annotations = map[string]string{}
		}
		in.Annotations[TolerateUnreadyEndpointsAnnotation] = "true"

		in.Spec.Selector = etcd.OffshootSelectors()
		in.Spec.Ports = core_util.MergeServicePorts(in.Spec.Ports, ports)

		in.Spec.ClusterIP = clusterIP
		if etcd.Spec.ServiceTemplate.Spec.Type != "" {
			in.Spec.Type = etcd.Spec.ServiceTemplate.Spec.Type
		}
		in.Spec.ExternalIPs = etcd.Spec.ServiceTemplate.Spec.ExternalIPs
		in.Spec.LoadBalancerIP = etcd.Spec.ServiceTemplate.Spec.LoadBalancerIP
		in.Spec.LoadBalancerSourceRanges = etcd.Spec.ServiceTemplate.Spec.LoadBalancerSourceRanges
		in.Spec.ExternalTrafficPolicy = etcd.Spec.ServiceTemplate.Spec.ExternalTrafficPolicy
		in.Spec.HealthCheckNodePort = etcd.Spec.ServiceTemplate.Spec.HealthCheckNodePort
		return in
	})
	return err
}
