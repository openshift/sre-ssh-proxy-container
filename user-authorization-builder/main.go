package main

import (
	"fmt"
	"log"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/openshift/user-authorization-builder/sresshd"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ldapHost = "ldap.corp.redhat.com"
	ldapPort = "389"
)

func main() {
	//Will hold the k8s resources to be put inside the SelectorSyncSet
	RawResource := make([]runtime.RawExtension, 0)

	//Create a connection to LDAP
	ldapConn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%s", ldapHost, ldapPort))
	if err != nil {
		log.Fatal("Error: ", err)
	}
	defer ldapConn.Close()

	//Get list of users
	users, err := sresshd.GetSREUsersList(ldapConn, "aos-sre")
	if err != nil {
		log.Fatal("Error: ", err)
	}

	//Get key for every user in the users list
	err = sresshd.GetSREUsersPubKeys(ldapConn, users)
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
