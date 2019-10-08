module kubedb.dev/etcd

go 1.12

require (
	github.com/appscode/go v0.0.0-20191006073906-e3d193d493fc
	github.com/cockroachdb/cmux v0.0.0-20170110192607-30d10be49292
	github.com/codeskyblue/go-sh v0.0.0-20190412065543-76bd3d59ff27
	github.com/coreos/etcd v3.3.13+incompatible
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f
	github.com/coreos/prometheus-operator v0.30.1
	github.com/ghodss/yaml v1.0.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.3-0.20190127221311-3c4408c8b829
	github.com/sirupsen/logrus v1.4.1
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	gomodules.xyz/stow v0.2.0
	google.golang.org/grpc v1.19.0
	k8s.io/api v0.0.0-20190503110853-61630f889b3c
	k8s.io/apiextensions-apiserver v0.0.0-20190516231611-bf6753f2aa24
	k8s.io/apimachinery v0.0.0-20190508063446-a3da69d3723c
	k8s.io/apiserver v0.0.0-20190516230822-f89599b3f645
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/kube-aggregator v0.0.0-20190314000639-da8327669ac5
	kmodules.xyz/client-go v0.0.0-20191006173540-91f8ee6b6b4b
	kmodules.xyz/custom-resources v0.0.0-20190927035424-65fe358bb045
	kmodules.xyz/monitoring-agent-api v0.0.0-20190808150221-601a4005b7f7
	kmodules.xyz/objectstore-api v0.0.0-20191006080053-fc8b57fadcf0
	kmodules.xyz/offshoot-api v0.0.0-20190901210649-de049192326c
	kmodules.xyz/webhook-runtime v0.0.0-20190808145328-4186c470d56b
	kubedb.dev/apimachinery v0.13.0-rc.1
)

replace (
	cloud.google.com/go => cloud.google.com/go v0.34.0
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.2+incompatible
	k8s.io/api => k8s.io/api v0.0.0-20190313235455-40a48860b5ab
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190315093550-53c4693659ed
	k8s.io/apimachinery => github.com/kmodules/apimachinery v0.0.0-20190508045248-a52a97a7a2bf
	k8s.io/apiserver => github.com/kmodules/apiserver v0.0.0-20190811223248-5a95b2df4348
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190314001948-2899ed30580f
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190314002645-c892ea32361a
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190314000054-4a91899592f4
	k8s.io/klog => k8s.io/klog v0.3.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190314000639-da8327669ac5
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20190228160746-b3a7cee44a30
	k8s.io/metrics => k8s.io/metrics v0.0.0-20190314001731-1bd6a4002213
	k8s.io/utils => k8s.io/utils v0.0.0-20190221042446-c2654d5206da
	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v0.0.0-20190302045857-e85c7b244fd2
)
