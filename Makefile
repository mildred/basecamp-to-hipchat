NAME=basecamp-to-hipchat
VERSION=$(shell git describe --always HEAD | sed -r 's/^[^0-9]//; s/[^0-9a-zA-Z]+/./g')

$(V)$(VERBOSE).SILENT:

help:
	echo "$(MAKE) dep            - Vendor dependencies"
	echo "$(MAKE) docker-image   - Build Docker image and export it"
	echo "$(MAKE) docker-package - Generate docker package using fpm"
	echo
	echo "NAME=$(NAME)"
	echo "VERSION=$(VERSION)"

dep:
	godep save -r

docker-image: $(NAME).tar
docker-package: $(NAME).deb

$(NAME).tar:
	docker build -t $(NAME) .
	docker save $(NAME) >$@
	docker rmi $(NAME)

$(NAME).deb: $(NAME).tar after-install.sh before-remove.sh
	fpm -s dir -t deb \
	--name $(NAME) \
	--version $(VERSION) \
	--after-install .tmp.after-install.sh \
	--before-remove .tmp.before-remove.sh \
	--prefix /var/lib/docker-images $(NAME).tar

.tmp.after-install.sh: Makefile
	: >$@
	echo "#!/bin/bash" >>$@
	echo "set -ex" >>$@
	echo "docker load -i /var/lib/docker-images/$(NAME).tar" >>$@
	echo "docker run -d --restart=always --name=$(NAME) $(NAME)" >>$@

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


.PHONY: help dep docker-image docker-package
