SHELL := /usr/bin/env bash

# Include shared Makefiles
include project.mk
include standard.mk

# Extend Makefile after here

.PHONY: local
local:
	$(CONTAINER_ENGINE) run --rm --publish 2222:2222 --interactive --tty --volume $(HOME)/.ssh/authorized_keys:/var/run/authorized_keys.d/authorized_keys --name sre-sshd $(IMAGE_URI_LATEST)
