SHELL := /usr/bin/env bash

# Include shared Makefiles
include project.mk
include standard.mk

# Extend Makefile after here

.PHONY: local
local:
	if [[ ! -e fake_ssh_host_rsa_key ]]; then ssh-keygen -q -t rsa -f fake_ssh_host_rsa_key -C '' -N '' >&/dev/null; fi
	$(CONTAINER_ENGINE) run --rm --publish 2222:2222 --interactive --tty \
	--volume $(HOME)/.ssh/authorized_keys:/var/run/authorized_keys.d/authorized_keys \
	--volume $(PWD)/fake_ssh_host_rsa_key:/opt/ssh_host_rsa_key \
	--env "AUTHORIZED_KEYS_DIR=/var/run/authorized_keys.d" \
	--env "SSH_HOST_RSA_KEY=/opt/ssh_host_rsa_key" \
	--name sre-sshd $(IMAGE_URI_LATEST)
	rm -f fake_ssh_host_rsa_key fake_ssh_host_rsa_key.pub
