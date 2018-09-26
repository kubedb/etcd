package admission

import (
	"net/http"
	"testing"

	"github.com/appscode/go/types"
	"github.com/appscode/kutil/meta"
	catalogapi "github.com/kubedb/apimachinery/apis/catalog/v1alpha1"
	dbapi "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	extFake "github.com/kubedb/apimachinery/client/clientset/versioned/fake"
	"github.com/kubedb/apimachinery/client/clientset/versioned/scheme"
	admission "k8s.io/api/admission/v1beta1"
	apps "k8s.io/api/apps/v1"
	authenticationV1 "k8s.io/api/authentication/v1"
	core "k8s.io/api/core/v1"
	storageV1beta1 "k8s.io/api/storage/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientSetScheme "k8s.io/client-go/kubernetes/scheme"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
)

func init() {
	scheme.AddToScheme(clientSetScheme.Scheme)
}

var requestKind = metaV1.GroupVersionKind{
	Group:   dbapi.SchemeGroupVersion.Group,
	Version: dbapi.SchemeGroupVersion.Version,
	Kind:    dbapi.ResourceKindEtcd,
}

func TestEtcdValidator_Admit(t *testing.T) {
	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			validator := EtcdValidator{}

			validator.initialized = true
			validator.extClient = extFake.NewSimpleClientset(
				&catalogapi.EtcdVersion{
					ObjectMeta: metaV1.ObjectMeta{
						Name: "3.2.13",
					},
				},
			)
			validator.client = fake.NewSimpleClientset(
				&core.Secret{
					ObjectMeta: metaV1.ObjectMeta{
						Name:      "foo-auth",
						Namespace: "default",
					},
				},
				&storageV1beta1.StorageClass{
					ObjectMeta: metaV1.ObjectMeta{
						Name: "standard",
					},
				},
			)

			objJS, err := meta.MarshalToJson(&c.object, dbapi.SchemeGroupVersion)
			if err != nil {
				panic(err)
			}
			oldObjJS, err := meta.MarshalToJson(&c.oldObject, dbapi.SchemeGroupVersion)
			if err != nil {
				panic(err)
			}

			req := new(admission.AdmissionRequest)

			req.Kind = c.kind
			req.Name = c.objectName
			req.Namespace = c.namespace
			req.Operation = c.operation
			req.UserInfo = authenticationV1.UserInfo{}
			req.Object.Raw = objJS
			req.OldObject.Raw = oldObjJS

			if c.heatUp {
				if _, err := validator.extClient.KubedbV1alpha1().Etcds(c.namespace).Create(&c.object); err != nil && !kerr.IsAlreadyExists(err) {
					t.Errorf(err.Error())
				}
			}
			if c.operation == admission.Delete {
				req.Object = runtime.RawExtension{}
			}
			if c.operation != admission.Update {
				req.OldObject = runtime.RawExtension{}
			}

			response := validator.Admit(req)
			if c.result == true {
				if response.Allowed != true {
					t.Errorf("expected: 'Allowed=true'. but got response: %v", response)
				}
			} else if c.result == false {
				if response.Allowed == true || response.Result.Code == http.StatusInternalServerError {
					t.Errorf("expected: 'Allowed=false', but got response: %v", response)
				}
			}
		})
	}

}

var cases = []struct {
	testName   string
	kind       metaV1.GroupVersionKind
	objectName string
	namespace  string
	operation  admission.Operation
	object     dbapi.Etcd
	oldObject  dbapi.Etcd
	heatUp     bool
	result     bool
}{
	{"Create Valid Etcd",
		requestKind,
		"foo",
		"default",
		admission.Create,
		sampleEtcd(),
		dbapi.Etcd{},
		false,
		true,
	},
	{"Create Invalid Etcd",
		requestKind,
		"foo",
		"default",
		admission.Create,
		getAwkwardEtcd(),
		dbapi.Etcd{},
		false,
		false,
	},
	{"Edit Etcd Spec.DatabaseSecret with Existing Secret",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editExistingSecret(sampleEtcd()),
		sampleEtcd(),
		false,
		true,
	},
	{"Edit Etcd Spec.DatabaseSecret with non Existing Secret",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editNonExistingSecret(sampleEtcd()),
		sampleEtcd(),
		false,
		false,
	},
	{"Edit Status",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editStatus(sampleEtcd()),
		sampleEtcd(),
		false,
		true,
	},
	{"Edit Spec.Monitor",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editSpecMonitor(sampleEtcd()),
		sampleEtcd(),
		false,
		true,
	},
	{"Edit Invalid Spec.Monitor",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editSpecInvalidMonitor(sampleEtcd()),
		sampleEtcd(),
		false,
		false,
	},
	{"Edit Spec.DoNotPause",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editSpecDoNotPause(sampleEtcd()),
		sampleEtcd(),
		false,
		true,
	},
	{"Delete Etcd when Spec.DoNotPause=true",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		sampleEtcd(),
		dbapi.Etcd{},
		true,
		false,
	},
	{"Delete Etcd when Spec.DoNotPause=false",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		editSpecDoNotPause(sampleEtcd()),
		dbapi.Etcd{},
		true,
		true,
	},
	{"Delete Non Existing Etcd",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		dbapi.Etcd{},
		dbapi.Etcd{},
		false,
		true,
	},
}

func sampleEtcd() dbapi.Etcd {
	return dbapi.Etcd{
		TypeMeta: metaV1.TypeMeta{
			Kind:       dbapi.ResourceKindEtcd,
			APIVersion: dbapi.SchemeGroupVersion.String(),
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
			Labels: map[string]string{
				dbapi.LabelDatabaseKind: dbapi.ResourceKindEtcd,
			},
		},
		Spec: dbapi.EtcdSpec{
			Version:     "3.2.13",
			Replicas:    types.Int32P(1),
			DoNotPause:  true,
			StorageType: dbapi.StorageTypeDurable,
			Storage: &core.PersistentVolumeClaimSpec{
				StorageClassName: types.StringP("standard"),
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse("100Mi"),
					},
				},
			},
			Init: &dbapi.InitSpec{
				ScriptSource: &dbapi.ScriptSourceSpec{
					VolumeSource: core.VolumeSource{
						GitRepo: &core.GitRepoVolumeSource{
							Repository: "https://github.com/kubedb/etcd-init-scripts.git",
							Directory:  ".",
						},
					},
				},
			},
			UpdateStrategy: apps.StatefulSetUpdateStrategy{
				Type: apps.RollingUpdateStatefulSetStrategyType,
			},
			TerminationPolicy: dbapi.TerminationPolicyPause,
		},
	}
}

func getAwkwardEtcd() dbapi.Etcd {
	etcd := sampleEtcd()
	etcd.Spec.Version = "3.0"
	return etcd
}

func editExistingSecret(old dbapi.Etcd) dbapi.Etcd {
	old.Spec.DatabaseSecret = &core.SecretVolumeSource{
		SecretName: "foo-auth",
	}
	return old
}

func editNonExistingSecret(old dbapi.Etcd) dbapi.Etcd {
	old.Spec.DatabaseSecret = &core.SecretVolumeSource{
		SecretName: "foo-auth-fused",
	}
	return old
}

func editStatus(old dbapi.Etcd) dbapi.Etcd {
	old.Status = dbapi.EtcdStatus{
		Phase: dbapi.DatabasePhaseCreating,
	}
	return old
}

func editSpecMonitor(old dbapi.Etcd) dbapi.Etcd {
	old.Spec.Monitor = &mona.AgentSpec{
		Agent: mona.AgentPrometheusBuiltin,
		Prometheus: &mona.PrometheusSpec{
			Port: 1289,
		},
	}
	return old
}

// should be failed because more fields required for COreOS Monitoring
func editSpecInvalidMonitor(old dbapi.Etcd) dbapi.Etcd {
	old.Spec.Monitor = &mona.AgentSpec{
		Agent: mona.AgentCoreOSPrometheus,
	}
	return old
}

func editSpecDoNotPause(old dbapi.Etcd) dbapi.Etcd {
	old.Spec.DoNotPause = false
	return old
}
