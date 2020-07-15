package sresshd

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	ldap "github.com/go-ldap/ldap/v3"
	corev1 "k8s.io/api/core/v1"

	// //"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/client-go/kubernetes"
	// "k8s.io/client-go/tools/clientcmd"
)

//GetSREUsersList will search LDAP for all memberUids of users
//identified by the groupFilter. Returns a list of users
func GetSREUsersList(connection *ldap.Conn, groupFilter string) ([]string, error) {

	var sreMemberList []string

	//Create LDAP search request
	searchRequest := ldap.NewSearchRequest(
		"ou=groups,dc=redhat,dc=com", // The base dn to search
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(cn=%s))", groupFilter), // The filter to apply
		[]string{"memberUid"},                  // A list attributes to retrieve (memberUid)
		nil,
	)

	//Execute LDAP search
	sr, err := connection.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	//Parse LDAP search Entries
	for _, entry := range sr.Entries {
		sreMemberList = entry.GetAttributeValues("memberUid")
	}

	//Check the sereMemberList is not empty
	if len(sreMemberList) == 0 && cap(sreMemberList) == 0 {
		return nil, errors.New("LDAP search returned empty")
	}

	return sreMemberList, nil
}

//GetSREUsersPubKey will search LDAP for public keys of a list of users.
func GetSREUsersPubKey(connection *ldap.Conn, users []string) (map[string]string, error) {

	userKeys := map[string]string{}
	userKey := ""
	//Create LDAP search request

	for _, user := range users {

		searchRequest := ldap.NewSearchRequest(
			"dc=redhat,dc=com", // The base dn to search
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			fmt.Sprintf("((uid=%s))", user), // The filter to apply
			[]string{"ipaSshPubKey"},        // A list attributes to retrieve
			nil,
		)

		//Execute LDAP search request
		sr, err := connection.Search(searchRequest)
		if err != nil {
			return nil, err
		}

		//Parse the search request entries
		for _, entry := range sr.Entries {
			userKey = entry.GetAttributeValue("ipaSshPubKey")
		}

		//if the key is defined for the user, add it to the map
		if userKey != "" {
			userKeys[user] = userKey
		}

	}

	return userKeys, nil
}

//BuildAuthorizedKeysFile creates an authorized_keys file for an organization (i.e rhmi, app-sre, srep, etc).
func BuildAuthorizedKeysFile(userKeys map[string]string, path, org string) (bool, error) {

	//Create authorize_keys file
	file, err := os.OpenFile(fmt.Sprintf("%s/%s_authorized_keys", path, org), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}

	//Close file
	defer file.Close()

	writer := bufio.NewWriter(file)

	//Write every key entry in a new line
	for _, key := range userKeys {
		_, _ = writer.WriteString(key + "\n\n")
	}
	writer.Flush()

	return true, nil

}

//CreateConfigMap will create a k8s configmap and return it
func CreateConfigMap(name, namespace string, annotations, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: annotations,
			Name:        name,
			Namespace:   namespace,
		},
		Data: data,
	}
}

//BuildConfigMapData builds the data to be put on the configMap from the authorized keys file
func BuildConfigMapData(org string) (map[string]string, error) {

	configMapData := make(map[string]string)

	authFile := fmt.Sprintf("%s_authorized_keys", org)
	//Open the authorize_keys file for reading
	file, err := os.OpenFile(authFile, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	//Start a scanner to read keys
	scanner := bufio.NewScanner(file)

	//Export file data into the configMapData
	for scanner.Scan() {
		configMapData[authFile] += scanner.Text() + "\n"
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return configMapData, nil
}
