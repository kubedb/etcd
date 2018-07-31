package docker

import (
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
)

const (
	ImageKubedbOperator = "operator"
	ImageEtcdOperator   = "etcd-operator"
	ImageEtcd           = "etcd"
	ImageEtcdTools      = "etcd-tools"
)

type Docker struct {
	// docker Registry
	Registry string
	// Exporter tag
	ExporterTag string
}

func (d Docker) GetImage(etcd *api.Etcd) string {
	return d.Registry + "/" + ImageEtcd
}

func (d Docker) GetImageWithTag(etcd *api.Etcd) string {
	return d.GetImage(etcd) + ":" + string(etcd.Spec.Version)
}

func (d Docker) GetOperatorImage(etcd *api.Etcd) string {
	return d.Registry + "/" + ImageKubedbOperator
}

func (d Docker) GetOperatorImageWithTag(etcd *api.Etcd) string {
	return d.GetOperatorImage(etcd) + ":" + d.ExporterTag
}

func (d Docker) GetToolsImage(etcd *api.Etcd) string {
	return d.Registry + "/" + ImageEtcdTools
}

func (d Docker) GetToolsImageWithTag(etcd *api.Etcd) string {
	return d.GetToolsImage(etcd) + ":" + string(etcd.Spec.Version)
}
