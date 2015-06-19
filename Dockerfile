# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

# Copy the local package files to the container's workspace.
ADD . /go/src/github.com/mildred/basecamp-to-hipchat

# Build the outyet command inside the container.
# (You may fetch or manage dependencies here,
# either manually or with a tool like "godep".)
RUN go install github.com/mildred/basecamp-to-hipchat

ENV BASECAMP_USER=
ENV BASECAMP_PASS=
ENV HIPCHAT_API_KEY=

ENTRYPOINT /usr/bin/env
CMD basecamp-to-hipchat
