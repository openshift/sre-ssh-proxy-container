package sresshd

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	ldap "github.com/go-ldap/ldap/v3"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

//LdapSearchExecuter executes a LDAP search and returns the result
func LdapSearchExecuter(connection *ldap.Conn, baseDn, filter string, searchAttrs []string) (*ldap.SearchResult, error) {
	searchRequest := ldap.NewSearchRequest(
		baseDn, // The base dn to search
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,      // The filter to apply
		searchAttrs, // A list attributes to retrieve (memberUid)
		nil,
	)

	return connection.Search(searchRequest)

}

//GetSREUsersList will search LDAP for all memberUids of users
//identified by the groupFilter. Returns a list of users
func GetSREUsersList(connection *ldap.Conn, groupFilter string) ([]string, error) {

	var (
		sreMemberList []string
		baseDn        = "ou=groups,dc=redhat,dc=com"
		srFilter      = fmt.Sprintf("(&(cn=%s))", groupFilter)
		srAttr        = []string{"memberUid"}
	)

	//Execute an Ldap search for users
	srResult, err := LdapSearchExecuter(connection, baseDn, srFilter, srAttr)
	if err != nil {
		return nil, err
	}

	//Parse LDAP search Entries
	for _, entry := range srResult.Entries {
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

	var (
		userKeys = map[string]string{}
		userKey  = ""
		baseDn   = "dc=redhat,dc=com"
		srAttr   = []string{"ipaSshPubKey"}
		srFilter = ""
	)

	for _, user := range users {

		//Rebuild a filter per user
		srFilter = fmt.Sprintf("((uid=%s))", user)

		//Execute an Ldap Search
		srResult, err := LdapSearchExecuter(connection, baseDn, srFilter, srAttr)
		if err != nil {
			return nil, err
		}

		//Parse the search request entries
		for _, entry := range srResult.Entries {
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
func BuildAuthorizedKeysFile(userKeys map[string]string, path string) (bool, error) {

	//Create authorized_keys file
	file, err := os.OpenFile(fmt.Sprintf("%s/authorized_keys", path), os.O_CREATE|os.O_WRONLY, 0644)
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
func CreateConfigMap(name, namespace string, annotations, labels, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: annotations,
			Labels:      labels,
			Name:        name,
			Namespace:   namespace,
		},
		Data: data,
	}
}

//BuildConfigMapData builds the data to be put on the configMap from the authorized keys file
func BuildConfigMapData() (map[string]string, error) {

	configMapData := make(map[string]string)

	authFile := "authorized_keys"
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

//CreateSelectorSyncSet builds the hive SelectorSyncSet
func CreateSelectorSyncSet(resources []runtime.RawExtension, labels, matchLabels map[string]string) *hivev1.SelectorSyncSet {
	return &hivev1.SelectorSyncSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SelectorSyncSet",
			APIVersion: "hive.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "sre-sshd-authorization-keys-sync",
			Labels: labels,
		},
		Spec: hivev1.SelectorSyncSetSpec{
			SyncSetCommonSpec: hivev1.SyncSetCommonSpec{
				ResourceApplyMode: hivev1.SyncResourceApplyMode,
				Resources:         resources,
			},
			ClusterDeploymentSelector: metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
		},
	}
}

//WriteSSSYaml writes a SSS into a yaml file
func WriteSSSYaml(sss *hivev1.SelectorSyncSet) error {
	//Create <sss_name>.yaml file
	file, err := os.OpenFile(fmt.Sprintf("deploy/%s.yaml", sss.GetName()), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	//Close file
	defer file.Close()

	writer := bufio.NewWriter(file)

	out, err := yaml.Marshal(sss)
	if err != nil {
		return err
	}

	//Write every key entry in a new line
	for _, line := range out {
		_, _ = writer.WriteString(string(line))
	}

	writer.Flush()

	return nil
}
