package main

import (
	"fmt"
	"log"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/openshift/user-authorization-builder/sresshd"
	"k8s.io/apimachinery/pkg/runtime"
)

func main() {
	//RawResource will hold the k8s resources to be put inside the SelectorSyncSet
	RawResource := make([]runtime.RawExtension, 0)

	//Create a connection to LDAP
	lcln := &sresshd.LdapCLient{
		BaseDn:       "ou=groups,dc=redhat,dc=com",
		SearchFilter: "(&(cn=aos-sre))",
		SearchAttrs:  []string{"memberUid"},
		Host:         "ldap.corp.redhat.com",
		Port:         "389",
		Protocol:     "tcp",
	}

	var err error

	lcln.ClnConn, err = ldap.Dial(lcln.Protocol, fmt.Sprintf("%s:%s", lcln.Host, lcln.Port))
	if err != nil {
		log.Fatal("Error: ", err)
	}
	defer lcln.ClnConn.Close()

	//Get list of users
	users, err := lcln.GetSREUsersList()
	if err != nil {
		log.Fatal("Error: ", err)
	}

	//change baseDN and filters for key searching
	lcln.BaseDn = "dc=redhat,dc=com"
	lcln.SearchAttrs = []string{"ipaSshPubKey"}

	//Get key for every user in the users list
	err = lcln.GetSREUsersPubKeys(users)
	if err != nil {
		log.Fatal("Error: ", err)
	}

	//Build authorized_keys file
	success, err := sresshd.BuildAuthorizedKeysFile(users, ".")
	if err != nil {
		log.Fatal("Error: ", err)
	}
	if success {
		fmt.Println("authorized_keys file built")
	}

	//Create a K8S configMap
	confMapData, err := sresshd.BuildConfigMapData()
	configMap := sresshd.CreateConfigMap("sshd-srep-keys-config", "CHOCOLATE", nil, nil, confMapData)

	//Create SSS
	RawResource = append(RawResource, runtime.RawExtension{Object: configMap})
	sss := sresshd.CreateSelectorSyncSet(RawResource, nil, nil)

	sssError := sresshd.WriteSSSYaml(sss)
	if sssError != nil {
		log.Fatal("Error writing SSS: ", sssError)
	}

	fmt.Println("Selector Sync Set written succesfully to deployed folder.")

}
