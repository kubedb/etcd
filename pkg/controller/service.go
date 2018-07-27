package controller

import (
	core_util "github.com/appscode/kutil/core/v1"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	svc := newEtcdServiceManifest(svcName, clusterIP, ports, etcd)
	ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd)
	if rerr != nil {
		return rerr
	}
	svc.ObjectMeta = core_util.EnsureOwnerReference(svc.ObjectMeta, ref)
	_, err := kubecli.CoreV1().Services(ns).Create(svc)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func newEtcdServiceManifest(svcName, clusterIP string, ports []v1.ServicePort, etcd *api.Etcd) *v1.Service {
	labels := etcd.OffshootLabels()
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Labels:    labels,
			Namespace: etcd.Namespace,
			Annotations: map[string]string{
				TolerateUnreadyEndpointsAnnotation: "true",
			},
		},
		Spec: v1.ServiceSpec{
			Ports:     ports,
			Selector:  labels,
			ClusterIP: clusterIP,
		},
	}
	return svc
}
