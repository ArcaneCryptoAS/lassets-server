# This Dockerfile is based on the official LND Dockerfile by roasbeef

FROM golang:1.13-alpine as builder

# The version of LND to install
ARG LND_VERSION

# Force Go to use the cgo based DNS resolver. This is required to ensure DNS
# queries required to connect to linked containers succeed.
ENV GODEBUG netdns=cgo

# Install dependencies and install/build lnd.
RUN apk add --no-cache --update alpine-sdk \
    git \
    make \
    &&  mkdir -p $GOPATH/src/github.com/lightningnetwork/lnd \
    &&  cd $GOPATH/src/github.com/lightningnetwork/lnd \
    &&  git clone https://github.com/lightningnetwork/lnd.git . \ 
    &&  git checkout "v$LND_VERSION" \
    &&  make \
    &&  make install tags="signrpc walletrpc chainrpc invoicesrpc routerrpc"

# Start a new, final image to reduce size.
FROM alpine as final

# Add bash and curl 
RUN apk add --no-cache \
    bash \
    curl 

# Copy the entrypoint script.
COPY "start-lnd.sh" .
RUN chmod a+x start-lnd.sh

# Copy the waiting script
COPY "wait-for-bitcoind.sh" .
RUN chmod a+x wait-for-bitcoind.sh

# Prepare logging dir for LND
RUN mkdir -p /data
RUN chmod a+rw /data

# user ID
ARG uid

# group ID
ARG gid

# Create new user, but only if the default user name
# is not overriden. If that is overriden, it's because
# we're running in a CI (or similar) environment. 
# This whole add-new-user thing is really a hacky 
# workaround to get permissions to work properly, 
# but if everything is running as root (as on CI), 
# this is not an issue. 
ARG UNAME=lnd
RUN if [[ $UNAME = "lnd" ]]; then addgroup -g $gid $UNAME; fi
RUN if [[ $UNAME = "lnd" ]]; then adduser -u $uid -G $UNAME -D -s /bin/bash $UNAME; fi

# Run as newly created user
USER $UNAME

# Expose lnd ports (server, alice, bob).
EXPOSE 9735 10009 10010

# Copy the binaries and entrypoint from the builder image.
COPY --from=builder /go/bin/lncli /bin/
COPY --from=builder /go/bin/lnd /bin/

