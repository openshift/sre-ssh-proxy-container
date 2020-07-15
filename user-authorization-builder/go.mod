module github.com/openshift/user-authorization-builder

go 1.13

require (
	github.com/go-ldap/ldap/v3 v3.2.2
	github.com/google/go-cmp v0.4.0 // indirect
	github.com/openshift/hive v1.0.5
	k8s.io/api v0.18.5
	k8s.io/apimachinery v0.18.5
)

replace (
	github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.0.0 // Pin non-versioned import to v22.0.0
	github.com/metal3-io/baremetal-operator => github.com/openshift/baremetal-operator v0.0.0-20200206190020-71b826cc0f0a // Use OpenShift fork
	github.com/metal3-io/cluster-api-provider-baremetal => github.com/openshift/cluster-api-provider-baremetal v0.0.0-20190821174549-a2a477909c1d // Pin OpenShift fork
	github.com/terraform-providers/terraform-provider-aws => github.com/openshift/terraform-provider-aws v1.60.1-0.20200526184553-1a716dcc0fa8 // Pin to openshift fork with tag v2.60.0-openshift-1
	github.com/terraform-providers/terraform-provider-azurerm => github.com/openshift/terraform-provider-azurerm v1.41.1-openshift-3 // Pin to openshift fork with IPv6 fixes
	go.etcd.io/etcd => go.etcd.io/etcd v0.0.0-20191023171146-3cf2f69b5738 // Pin to version used by k8s.io/apiserver
	google.golang.org/api => google.golang.org/api v0.13.0 // Pin to version required by tf-provider-google
	google.golang.org/grpc => google.golang.org/grpc v1.23.1 // Pin to version used by k8s.io/apiserver
	k8s.io/client-go => k8s.io/client-go v0.18.2 // Pinned to keep from using an older v12.0.0 version that go mod thinks is newer
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20200121204235-bf4fb3bd569c // Pin to version used by k8s.io/apiserver
	sigs.k8s.io/cluster-api-provider-aws => github.com/openshift/cluster-api-provider-aws v0.2.1-0.20200506073438-9d49428ff837 // Pin OpenShift fork
	sigs.k8s.io/cluster-api-provider-azure => github.com/openshift/cluster-api-provider-azure v0.1.0-alpha.3.0.20200120114645-8a9592f1f87b // Pin OpenShift fork
	sigs.k8s.io/cluster-api-provider-openstack => github.com/openshift/cluster-api-provider-openstack v0.0.0-20200526112135-319a35b2e38e // Pin OpenShift fork
)
