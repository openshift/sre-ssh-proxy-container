package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"

	ldap "github.com/go-ldap/ldap/v3"
	corev1 "k8s.io/api/core/v1"

	// //"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/client-go/kubernetes"
	// "k8s.io/client-go/tools/clientcmd"
)

const (
	ldapHost = "ldap.corp.redhat.com"
	ldapPort = "389"
	baseDN   = "dc=redhat,dc=com"
)

//GetSREUsersList will search LDAP for all memberUids of users
//identified by the groupFilter. Returns a list of users
func GetSREUsersList(connection *ldap.Conn, groupFilter string) ([]string, error) {

	var sreMemberList []string

	//Create LDAP search request
	searchRequest := ldap.NewSearchRequest(
		fmt.Sprintf("ou=groups,%s", baseDN), // The base dn to search
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

//GetSREUserPubKey will search LDAP for a user's public key.
//Returns the public key or emptry string if no key is defined for the user.
func GetSREUserPubKey(connection *ldap.Conn, user string) (string, error) {
	var userKey string

	//Create LDAP search request
	searchRequest := ldap.NewSearchRequest(
		fmt.Sprintf("dc=redhat,dc=com"), // The base dn to search
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("((uid=%s))", user), // The filter to apply
		[]string{"ipaSshPubKey"},        // A list attributes to retrieve
		nil,
	)

	//Execute LDAP search request
	sr, err := connection.Search(searchRequest)
	if err != nil {
		return "", err
	}

	//Parse the search request entries
	for _, entry := range sr.Entries {
		userKey = entry.GetAttributeValue("ipaSshPubKey")
	}

	return userKey, nil
}

//BuildAuthorizedKeysFile creates an authorized_keys file.
//Returns success=true if the file was written
func BuildAuthorizedKeysFile(keys map[string]string, path string) (bool, error) {

	//Create authorize_keys file
	file, err := os.OpenFile(fmt.Sprintf("%s/authorized_keys", path), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}

	//Close file
	defer file.Close()

	writer := bufio.NewWriter(file)

	//Write every key entry in a new line
	for _, key := range keys {
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

//SshdConfigMapData builds the data to be put on the configMap from the authorized keys file
func sshdConfigMapData() (map[string]string, error) {

	configMapData := make(map[string]string)
	configMapData["authorized_keys"] = ""

	//Open the authorize_keys file for reading
	file, err := os.OpenFile("authorized_keys", os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	//Start a scanner to read keys
	scanner := bufio.NewScanner(file)

	//Export file data into the configMapData
	for scanner.Scan() {
		configMapData["authorized_keys"] += scanner.Text() + "\n"
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return configMapData, nil
}

func main() {

	userKeys := map[string]string{} //Map user=>PubKey

	//Create a connection to LDAP
	ldapConn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%s", ldapHost, ldapPort))
	if err != nil {
		log.Fatal("Error: ", err)
	}
	defer ldapConn.Close()

	//Get list of users
	//TODO: "aos-sre" param should not be hardcoded. to accept other teams as filter
	users, err := GetSREUsersList(ldapConn, "aos-sre")
	if err != nil {
		log.Fatal("Error: ", err)
	}

	//Get key for every user in the users list
	for _, user := range users {
		//get the user public key from LDAP
		pubKey, err := GetSREUserPubKey(ldapConn, user)
		if err != nil {
			log.Fatal("Error: ", err)
		}

		//Create user=>PubKey entry on map
		if pubKey != "" {
			userKeys[user] = pubKey
		}

	}

	//Build authorized_keys file
	success, err := BuildAuthorizedKeysFile(userKeys, ".")
	if err != nil {
		log.Fatal("Error: ", err)
	}
	if success {
		fmt.Println("authorized_keys file built")
	}

	//Create a K8S configMap
	confMapData, err := sshdConfigMapData()
	configMap := CreateConfigMap("sshd-srep-keys-config", "TODO", nil, confMapData)

	fmt.Println(configMap)

	//Create SSS

}
