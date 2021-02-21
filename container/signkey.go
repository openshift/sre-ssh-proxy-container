package main

import (
	"bytes"
	"context"
	"crypto"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"os"
	"time"

	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"flag"

	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type sshfp [sha256.Size]byte

type certSigner struct {
	crt *x509.Certificate
	key *rsa.PrivateKey
}

var kubeconfig string
var idKeyArg string
var signkeyArg string

func main() {

	var idKey ssh.PublicKey
	var signKey crypto.PublicKey
	var key []byte // client key
	var crt []byte // client cert

	authKeysArg := map[string]string{
		os.ExpandEnv("${AUTHORIZED_KEYS_DIR}/aos-sre/authorized_keys"): "osd-sre-admins",
	}

	log.SetFlags(log.Lshortfile)

	flag.StringVar(&kubeconfig, "kubeconfig", kubeconfig,
		"Path to kubeconfig file.")
	flag.StringVar(&idKeyArg, "id-key", "",
		"The public identity key to verify (SSH format). Required.")
	flag.StringVar(&signkeyArg, "sign-key", "",
		"Optional public key to sign (PEM format in base64). If not specified, a new keypair is generated.")
/*
	flag.Var(kflag.NewMapStringString(&authKeysArg), "auth-keys-file",
		"List of authorized_keys file and user group (eg: --auth-key-file /run/authorized_keys=osd-sre-admins")
*/
	flag.Parse()

	authorizedKeys := map[sshfp]string{}
	for authkeys, usergroup := range authKeysArg {
		for _, fingerprint := range parseAuthorizedKeysFile(authkeys) {
			authorizedKeys[fingerprint] = usergroup
		}
	}

	idKey, err := parseIDKey(idKeyArg)
	if err != nil {
		log.Fatalln(err)
	}

	signKey, err = parseSignKey(signkeyArg)
	if err != nil {
		log.Fatalln(err)
	}

	err = verifyRequest(idKey, &signKey)
	if err != nil {
		log.Fatalln(err)
	}

	if signKey == nil {
		signKey, key, err = generateKeypair()
		if err != nil {
			log.Fatalln("failed to generate a new keypair")
		}
	}

	id := fingerprint(idKey)

	if usergroup, ok := authorizedKeys[id]; ok {

		signer, err := getClusterSigner(kubeconfig, id)
		if err != nil {
			log.Fatalln(err)
		}

		crt, err = signer.Sign(id, signKey, usergroup)
		if err != nil {
			log.Fatalln(err)
		}

	} else {

		log.Fatalln("id key wasn't found in any authorized_keys file")

	}

	if key != nil {
		os.Stdout.Write(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}))
	}
	os.Stdout.Write(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: crt}))
	os.Stdout.Sync()

}

// if a forwarded agent is present, use it to verify ownership of id key
// if no agent is present, confirm sign key is unset or matches the id key
func verifyRequest(idkey ssh.PublicKey, signkey *crypto.PublicKey) error {

	if sock, ok := os.LookupEnv("SSH_AUTH_SOCK"); ok {

		conn, err := net.Dial("unix", sock)
		if err != nil {
			return fmt.Errorf("couldn't connect to agent: %s", err)
		}

		b := make([]byte, 32)
		if n, err := rand.Read(b); n != 32 || err != nil {
			return fmt.Errorf("unable to generate challenge")
		}

		sig, err := agent.NewClient(conn).Sign(idkey, b)
		if err != nil {
			return fmt.Errorf("couldn't verify key: %s", err)
		}

		err = idkey.Verify(b, sig)
		if err != nil {
			return fmt.Errorf("signature invalid: %s", err)
		}

		return nil

	}

	cryptoidkey := idkey.(ssh.CryptoPublicKey).CryptoPublicKey()
	cryptosignkey := *signkey

	switch keytype := cryptosignkey.(type) {
	case nil: // no sign key was given, so just use the id key
		*signkey = cryptoidkey
		return nil
	case rsa.PublicKey:
		if cryptosignkey.(*rsa.PublicKey).Equal(cryptoidkey) {
			return nil
		}
	case ed25519.PublicKey:
		if cryptosignkey.(ed25519.PublicKey).Equal(cryptoidkey) {
			return nil
		}
	default: 
		return fmt.Errorf("unsupported key type %q", keytype)
	}

	return fmt.Errorf("no agent available to verify id key")

}

// generates a 24-hour certificate with the id key fingerprint and usergroup in its DN
func (s *certSigner) Sign(id sshfp, signKey crypto.PublicKey, usergroup string) ([]byte, error) {

	username := "redhat-" + usergroup + "-" + base58.Encode(id[:12])
	b64fp := base64.RawStdEncoding.EncodeToString(id[:])

	template := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: username,
			Organization: []string{
				usergroup,
				"ssh:sha256:" + b64fp, // ensures full id key fingerprint ends up in audit logs
				"system:authenticated:ssh",
			},
			OrganizationalUnit: []string{fmt.Sprintf("OpenShift Dedicated")},
		},
		SerialNumber:       big.NewInt(time.Now().UnixNano()),
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		NotBefore:          time.Now().Add(time.Hour * -1),
		NotAfter:           time.Now().Add(time.Hour * 24),
		SignatureAlgorithm: s.crt.SignatureAlgorithm,
	}

	c, err := x509.CreateCertificate(rand.Reader, template, s.crt, signKey, s.key)
	if err != nil {
		return nil, fmt.Errorf("error creating cert: %s", err)
	}

	return c, nil

}

// extract public key from ssh wire format (eg: "ssh-rsa AAAAB3NzaC1yc2E...")
func parseIDKey(idkey string) (ssh.PublicKey, error) {

	if idkey == "" {
		return nil, fmt.Errorf("missing identity key")
	}

	sshkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(idkey))
	if err != nil {
		return nil, fmt.Errorf("bad identity key: %s", err)
	}

	return sshkey, nil

}

// extract public key from single-line base64-encoded PEM block
func parseSignKey(signkey string) (crypto.PublicKey, error) {

	if len(signkey) == 0 {
		return nil, nil
	}

	data, err := base64.RawStdEncoding.DecodeString(signkey)
	if err != nil {
		return nil, fmt.Errorf("couldn't decode sign key: %s", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("bad sign key encoding (expected PEM)")
	}

	var key crypto.PublicKey

	switch block.Type {
	case "RSA PUBLIC KEY":
		key, err = x509.ParsePKCS1PublicKey(block.Bytes)
	case "PUBLIC KEY": // rsa, dsa, ecdsa, ed25519
		key, err = x509.ParsePKIXPublicKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unexpected key type %s", block.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("bad sign key: %s", err)
	}

	return key, nil

}

// returns a slice of fingerprints from an authorized_keys file
func parseAuthorizedKeysFile(path string) []sshfp {

	fingerprints := []sshfp{}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %s", path, err)
		return fingerprints
	}

	for _, line := range bytes.Split(data, []byte{0x0a}) {
		if len(line) == 0 {
			continue
		}
		pubkey, err := parseIDKey(string(line))
		if err != nil {
			fmt.Fprintf(os.Stderr, "couldn't parse key at line %d of %s: %s\n", line, path, err)
			continue
		}
		fingerprints = append(fingerprints, fingerprint(pubkey))
	}

	return fingerprints

}

// sha256 hash as a fixed-length slice to use as map key
func fingerprint(pubkey ssh.PublicKey) sshfp {

	var out [sha256.Size]byte
	hash := sha256.New()
	_, err := hash.Write(pubkey.Marshal())
	if  err != nil {
		log.Fatalln("couldn't generate fingerprint:", err)
	}
	sum := hash.Sum(nil)[:]
	copy(out[:sha256.Size], sum[:])
	return out

}

// generate a new keypair, convert private key to PEM
func generateKeypair() (crypto.PublicKey, []byte, error) {

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	b, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, nil, err
	}

	return public, b, nil

}

// fetches the current csr signer secret
func getClusterSigner(kubeconfig string, id sshfp) (*certSigner, error) {

	var config *rest.Config
	var err error

	if kubeconfig == "" {
		config, err = rest.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		log.Fatalln("not running in-cluster and no kubeconfig specified")
	}

	// add some additional info to the audit logs
	hostname, _ := os.Hostname()
	config.UserAgent = fmt.Sprintf("signkey/0.0.0 (pod: %s; pid: %d; fp: %s)",
		hostname, os.Getpid(), base64.RawStdEncoding.EncodeToString(id[:]))

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalln("couldn't build kube client:", err)
	}

	csrSecret, err := client.CoreV1().Secrets("openshift-kube-controller-manager").Get(
		context.TODO(), "csr-signer", v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("couldn't get signer secret: %s", err)
	}

	certData, _ := pem.Decode(csrSecret.Data["tls.crt"])
	signerCert, err := x509.ParseCertificate(certData.Bytes)
	if err != nil {
		log.Fatalln("couldn't parse signer cert:", err)
	}

	keyData, _ := pem.Decode(csrSecret.Data["tls.key"])
	signerKey, err := x509.ParsePKCS1PrivateKey(keyData.Bytes)
	if err != nil {
		log.Fatalln("couldn't parse signer key:", err)
	}

	return &certSigner{signerCert, signerKey}, nil

}

