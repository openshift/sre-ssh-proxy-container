package sresshd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"

	ldap "github.com/go-ldap/ldap/v3"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

//SreUser describes an SRE information
type SreUser struct {
	ID     string
	PubKey string
}

//LdapCLient describes all the information needed to query ldap
type LdapCLient struct {
	ClnConn      *ldap.Conn
	Protocol     string
	Host         string
	Port         string
	BaseDn       string
	SearchFilter string
	SearchAttrs  []string
}

//ldapSearchExecuter executes a LDAP search and returns the result
func (l *LdapCLient) ldapSearchExecuter() (*ldap.SearchResult, error) {
	searchRequest := ldap.NewSearchRequest(
		l.BaseDn, // The base dn to search
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		l.SearchFilter, // The filter to apply
		l.SearchAttrs,  // A list attributes to retrieve
		nil,
	)

	return l.ClnConn.Search(searchRequest)
}

//GetSREUsersList will search LDAP for all memberUids on a group
func (l *LdapCLient) GetSREUsersList() ([]SreUser, error) {

	userList := []SreUser{}
	ldapUsers := []string{}

	//Execute an Ldap search for users
	srResult, err := l.ldapSearchExecuter()
	if err != nil {
		return nil, err
	}

	//Parse LDAP search Entries
	for _, entry := range srResult.Entries {
		//get ldapUsers in the results
		ldapUsers = entry.GetAttributeValues(strings.Join(l.SearchAttrs, ","))

		//for Every ID in ldapUsers create a sre member and append it to the list
		sre := SreUser{}
		for _, id := range ldapUsers {
			sre.ID = id
			userList = append(userList, sre)
		}
	}

	//Check the userList is not empty
	if len(userList) == 0 {
		s := fmt.Sprintf("LDAP Search returned no users for group: %s", l.SearchFilter)
		return nil, errors.New(s)
	}

	return userList, nil
}

//GetSREUsersPubKeys will search LDAP for public keys of a list of users.
func (l *LdapCLient) GetSREUsersPubKeys(sreUsersList []SreUser) error {

	for idx, sreUser := range sreUsersList {

		//Rebuild a filter per user
		l.SearchFilter = fmt.Sprintf("((uid=%s))", sreUser.ID)
		fmt.Println(l.SearchFilter)
		//Execute an Ldap Search
		srResult, err := l.ldapSearchExecuter()
		if err != nil {
			return err
		}

		//Parse the search request entries
		for _, entry := range srResult.Entries {
			sreUsersList[idx].PubKey = entry.GetAttributeValue(strings.Join(l.SearchAttrs, ","))
			fmt.Println(sreUsersList[idx].PubKey)
		}

	}
	fmt.Println(sreUsersList)

	return nil
}

//BuildAuthorizedKeysFile creates an authorized_keys file
func BuildAuthorizedKeysFile(sreUser []SreUser, path string) (bool, error) {

	//Create authorized_keys file
	file, err := os.OpenFile(fmt.Sprintf("%s/authorized_keys", path), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}

	//Close file
	defer file.Close()

	writer := bufio.NewWriter(file)

	//Write every key entry in a new line
	for _, user := range sreUser {
		if len(user.PubKey) > 0 {
			_, _ = writer.WriteString(user.PubKey + "\n\n")
		}
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

//BuildConfigMapData builds the data to be put on the configMap from the authorized_keys file
func BuildConfigMapData(path string) (map[string]string, error) {

	configMapData := make(map[string]string)

	authFile := fmt.Sprintf("%s/authorized_keys", path)
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
