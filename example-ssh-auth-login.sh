#!/bin/bash

if [ -z "$1" ] || [[ "$1" == "-?-h*" ]]; then
	echo "usage: $0 [ id | external_id | name | url | rh-ssh host ]"
fi

SEARCH="managed='t' AND (id LIKE '$1' OR name LIKE '$1' OR external_id LIKE '$1' OR api.url LIKE '%$1%')"

if [[ "$1" == "rh-ssh."* ]]; then
	SSH_HOST=$1
else
	SSH_HOST=$(ocm get clusters --parameter size=10 --parameter search="${SEARCH}" \
		| jq -r  '.items[]|.api.url|sub("^[^\\.]*";"rh-ssh")|gsub("(:|/).*";"")' 2>/dev/null)
fi

if [ -z "$SSH_HOST" ]; then
	echo "no matching cluster found" && exit 1
fi

set -- $SSH_HOST

if [ ${#@} -gt 1 ]; then
	select host; do
		export SSH_HOST=$host && break
	done
fi

tmp=$(mktemp -u)

export SIGN_KEY="$(openssl genpkey -algorithm ED25519 | tee $tmp.key | openssl pkey -pubout | openssl enc -a -A -nopad)"

echo "running signkey on ssh host $SSH_HOST..."

ssh -A -o SetEnv=SIGN_KEY="${SIGN_KEY//=/}" sre-user@$SSH_HOST signkey \
	| tee $tmp.crt | openssl x509 -subject -enddate -issuer -noout 2>/dev/null

if [ $? -ne 0 ]; then
	exit 1
fi

{	SERVER=${SSH_HOST/rh-ssh/api}:6443
	CLUSTER=${SERVER//./-}
	oc config set-cluster $CLUSTER --server https://$SERVER
	oc config set-credentials user/$CLUSTER --client-key=$tmp.key --client-certificate=$tmp.crt --embed-certs
	oc config set-context default/$CLUSTER/user --cluster $CLUSTER --user user/$CLUSTER --namespace default
	oc config use-context default/$CLUSTER/user
	rm $tmp.*
} >/dev/null

echo logged into $(oc whoami --show-server) as $(oc whoami)
