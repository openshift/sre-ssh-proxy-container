package main

import (
	"fmt"
	"log"
	"os"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/openshift/user-authorization-builder/sresshd"
	"k8s.io/apimachinery/pkg/runtime"
)

func main() {

	//Get the groups filters passed to the script
	var groupFilters []string
	//Exit if no group filters were found
	if len(os.Args) > 1 {
		groupFilters = os.Args[1:]
	} else {
		log.Fatal("Err: No group filters have been identified.")
	}

	//TODO: for every group in group filter { execute entire script to produce a SSS per group}
	//TODO: Modifs. (Each authorized keys files will have to have a suffix with the group name)
	//or we will need to build the ConfigMap Data directy from the map and not from the auth_keys files.

	//RawResource will hold the k8s resources to be put inside the SelectorSyncSet
	for _, group := range groupFilters {

		RawResource := make([]runtime.RawExtension, 0)
		var err error
		//Create a connection to LDAP
		lcln := &sresshd.LdapCLient{
			BaseDn:       "ou=groups,dc=redhat,dc=com",
			SearchFilter: fmt.Sprintf("(&(cn=%s))", group),
			SearchAttrs:  []string{"memberUid"},
			Host:         "ldap.corp.redhat.com",
			Port:         ldap.DefaultLdapPort,
			Protocol:     "tcp",
		}

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
		success, err := sresshd.BuildAuthorizedKeysFile(users, group, "keys")
		if err != nil {
			log.Fatal("Error: ", err)
		}
		if success {
			fmt.Println("authorized_keys file built")
		}

		//Create a K8S configMap
		confMapData, err := sresshd.BuildConfigMapData("keys", group)
		configMap := sresshd.CreateConfigMap("sshd-srep-keys-config", group, "FIXME", nil, nil, confMapData)

		//Create SSS
		RawResource = append(RawResource, runtime.RawExtension{Object: configMap})
		sss := sresshd.CreateSelectorSyncSet(RawResource, group, nil, nil)

		sssError := sresshd.WriteSSSYaml(sss)
		if sssError != nil {
			log.Fatal("Error writing SSS: ", sssError)
		}

		fmt.Printf("SelectorSyncSet for %s written succesfully in deployed folder.\n", group)
	}
}
