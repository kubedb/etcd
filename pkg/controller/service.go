package controller

import (
	"fmt"

	mon_api "github.com/appscode/kube-mon/api"
	"github.com/appscode/kutil"
	core_util "github.com/appscode/kutil/core/v1"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/pkg/eventer"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
)

const TolerateUnreadyEndpointsAnnotation = "service.alpha.kubernetes.io/tolerate-unready-endpoints"

func (c *Controller) ensureService(etcd *api.Etcd) (kutil.VerbType, error) {
	// Check if service name exists
	if err := c.checkService(etcd); err != nil {
		return kutil.VerbUnchanged, err
	}

	// create database Service
	vt, err := c.createClientService(etcd)
	if err != nil {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
			c.recorder.Eventf(
				ref,
				core.EventTypeWarning,
				eventer.EventReasonFailedToCreate,
				"Failed to create Service. Reason: %v",
				err,
			)
		}
		return kutil.VerbUnchanged, err
	} else if vt != kutil.VerbUnchanged {
		if ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd); rerr == nil {
			c.recorder.Eventf(
				ref,
				core.EventTypeNormal,
				eventer.EventReasonSuccessful,
				"Successfully %s Service",
				vt,
			)
		}
	}

	return vt, nil
}

func (c *Controller) checkService(etcd *api.Etcd) error {
	name := etcd.OffshootName() + "-client"
	service, err := c.Client.CoreV1().Services(etcd.Namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return nil
		}
		return err
	}

	if service.Spec.Selector[api.LabelDatabaseName] != name {
		return fmt.Errorf(`intended service "%v" already exists`, name)
	}

	return nil
}

func (c *Controller) createClientService(etcd *api.Etcd) (kutil.VerbType, error) {
	meta := metav1.ObjectMeta{
		Name:      etcd.OffshootName() + "-client",
		Namespace: etcd.Namespace,
		Annotations: map[string]string{
			TolerateUnreadyEndpointsAnnotation: "true",
		},
	}

	ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd)
	if rerr != nil {
		return kutil.VerbUnchanged, rerr
	}

	_, ok, err := core_util.CreateOrPatchService(c.Client, meta, func(in *core.Service) *core.Service {
		in.ObjectMeta = core_util.EnsureOwnerReference(in.ObjectMeta, ref)
		in.Labels = etcd.OffshootLabels()
		in.Spec.Ports = upsertClientServicePort(in, etcd)
		in.Spec.Selector = etcd.OffshootLabels()
		return in
	})
	return ok, err
}

func (c *Controller) createPeerService(etcd *api.Etcd) (kutil.VerbType, error) {
	meta := metav1.ObjectMeta{
		Name:      etcd.OffshootName(),
		Namespace: etcd.Namespace,
		Annotations: map[string]string{
			TolerateUnreadyEndpointsAnnotation: "true",
		},
	}

	ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd)
	if rerr != nil {
		return kutil.VerbUnchanged, rerr
	}

	_, ok, err := core_util.CreateOrPatchService(c.Client, meta, func(in *core.Service) *core.Service {
		in.ObjectMeta = core_util.EnsureOwnerReference(in.ObjectMeta, ref)
		in.Labels = etcd.OffshootLabels()
		in.Spec.Ports = upsertPeerServicePort(in, etcd)
		in.Spec.Selector = etcd.OffshootLabels()
		in.Spec.ClusterIP = core.ClusterIPNone
		in.Spec.Type = core.ServiceTypeClusterIP
		return in
	})
	return ok, err
}

func upsertClientServicePort(service *core.Service, etcd *api.Etcd) []core.ServicePort {
	desiredPorts := []core.ServicePort{
		{
			Name:       "client",
			Protocol:   core.ProtocolTCP,
			Port:       2379,
			TargetPort: intstr.FromInt(2379),
		},
	}
	if etcd.GetMonitoringVendor() == mon_api.VendorPrometheus {
		desiredPorts = append(desiredPorts, core.ServicePort{
			Name:       api.PrometheusExporterPortName,
			Protocol:   core.ProtocolTCP,
			Port:       etcd.Spec.Monitor.Prometheus.Port,
			TargetPort: intstr.FromString(api.PrometheusExporterPortName),
		})
	}
	return core_util.MergeServicePorts(service.Spec.Ports, desiredPorts)
}

func upsertPeerServicePort(service *core.Service, etcd *api.Etcd) []core.ServicePort {
	desiredPorts := []core.ServicePort{
		{
			Name:       "client",
			Protocol:   core.ProtocolTCP,
			Port:       2379,
			TargetPort: intstr.FromInt(2379),
		},
		{
			Name:       "peer",
			Port:       2380,
			TargetPort: intstr.FromInt(2380),
			Protocol:   core.ProtocolTCP,
		},
	}
	if etcd.GetMonitoringVendor() == mon_api.VendorPrometheus {
		desiredPorts = append(desiredPorts, core.ServicePort{
			Name:       api.PrometheusExporterPortName,
			Protocol:   core.ProtocolTCP,
			Port:       etcd.Spec.Monitor.Prometheus.Port,
			TargetPort: intstr.FromString(api.PrometheusExporterPortName),
		})
	}
	return core_util.MergeServicePorts(service.Spec.Ports, desiredPorts)
}

func (c *Controller) createEtcdGoverningService(etcd *api.Etcd) (string, error) {
	ref, rerr := reference.GetReference(clientsetscheme.Scheme, etcd)
	if rerr != nil {
		return "", rerr
	}

	service := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcd.ServiceName(),
			Labels:    etcd.OffshootLabels(),
			Namespace: etcd.Namespace,
			Annotations: map[string]string{
				TolerateUnreadyEndpointsAnnotation: "true",
			},
		},
		Spec: core.ServiceSpec{
			Type:      core.ServiceTypeClusterIP,
			ClusterIP: core.ClusterIPNone,
			Ports: []core.ServicePort{
				{
					Name:       "client",
					Protocol:   core.ProtocolTCP,
					Port:       2379,
					TargetPort: intstr.FromInt(2379),
				},
				{
					Name:       "peer",
					Port:       2380,
					TargetPort: intstr.FromInt(2380),
					Protocol:   core.ProtocolTCP,
				},
			},
			Selector: etcd.OffshootLabels(),
		},
	}
	service.ObjectMeta = core_util.EnsureOwnerReference(service.ObjectMeta, ref)

	_, err := c.Client.CoreV1().Services(etcd.Namespace).Create(service)
	if err != nil && !kerr.IsAlreadyExists(err) {
		return "", err
	}
	return service.Name, nil
}
