module github.com/openshift/sre-ssh-proxy-container

go 1.15

require (
	github.com/btcsuite/btcutil v1.0.2
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/component-base v0.20.4
)

replace k8s.io/client-go => k8s.io/client-go v0.20.4
