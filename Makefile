NAME=basecamp-to-hipchat
VERSION=$(shell git describe --always HEAD | sed -r 's/^[^0-9]//; s/[^0-9a-zA-Z]+/./g')

-include config.mk

SERVER_USER?=root
SERVER_PORT?=22

$(V)$(VERBOSE).SILENT:

help:
	echo "$(MAKE) deps                  - Vendor dependencies"
	echo "$(MAKE) docker-image          - Build Docker image and export it"
	echo "$(MAKE) docker-package        - Generate docker package using fpm"
	echo "$(MAKE) deploy                - Deploy to SERVER_USER@SERVER:SERVER_PORT"
	echo "$(MAKE) package-and-deploy    - Run docker-package and deploy"
	echo
	echo "NAME=$(NAME)"
	echo "VERSION=$(VERSION)"
	echo "SERVER=$(SERVER)"
	echo "SERVER_USER=$(SERVER_USER)"
	echo "SERVER_PORT=$(SERVER_PORT)"

deps:
	godep save -r

docker-image:
	$(MAKE) -B $(NAME).tar

docker-package: docker-deb-package

$(NAME).tar: Dockerfile
	docker build -t $(NAME) .
	docker save $(NAME) >$@
	docker rmi $(NAME)

$(NAME).env: Makefile
	: >$@
	echo "BASECAMP_USER=$(BASECAMP_USER)" >>$@
	echo "BASECAMP_PASS=$(BASECAMP_PASS)" >>$@
	echo "HIPCHAT_API_KEY=$(HIPCHAT_API_KEY)" >>$@

docker-deb-package: $(NAME).tar $(NAME).env .tmp.after-install.sh .tmp.before-remove.sh
	fpm -f -s dir -t deb \
	--name $(NAME) \
	--version $(VERSION) \
	--after-install .tmp.after-install.sh \
	--before-remove .tmp.before-remove.sh \
	--config-files /etc/$(NAME).env \
	$(NAME).env=/etc/$(NAME).env \
	$(NAME).tar=/var/lib/docker-images/$(NAME).tar

.tmp.after-install.sh: Makefile
	: >$@
	echo "#!/bin/bash" >>$@
	echo "set -ex" >>$@
	echo "docker load -i /var/lib/docker-images/$(NAME).tar" >>$@
	echo "docker run -d --restart=always --env-file=/etc/$(NAME).env --name=$(NAME) $(NAME)" >>$@

.tmp.before-remove.sh: Makefile
	: >$@
	echo "#!/bin/bash" >>$@
	echo "( set -x" >>$@
	echo "  docker kill $(NAME)" >>$@
	echo "  docker rm $(NAME)" >>$@
	echo ")" >>$@
	echo "if docker images | grep '^$(NAME) '; then" >>$@
	echo "( set -ex" >>$@
	echo "  docker rmi $(NAME)" >>$@
	echo ")" >>$@
	echo "fi" >>$@
	echo "true" >>$@

deploy:
	scp -P $(SERVER_PORT) $(NAME)_$(VERSION)_*.deb $(SERVER_USER)@$(SERVER):/tmp/$(NAME)_$(VERSION).deb
	ssh -p $(SERVER_PORT) $(SERVER_USER)@$(SERVER) 'dpkg -i /tmp/$(NAME)_$(VERSION).deb && rm /tmp/$(NAME)_$(VERSION).deb'

package-and-deploy:
	$(MAKE) docker-package
	$(MAKE) deploy

.PHONY: help dep docker-image docker-package docker-deb-package deploy package-and-deploy
