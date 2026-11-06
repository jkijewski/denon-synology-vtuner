# Test image: runs vtunerd the way the Synology package does, without DSM.
# Used by compose.yaml together with nginx, which plays the part of DSM's reverse proxy.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/vtunerd ./cmd/vtunerd \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/fakereceiver ./cmd/fakereceiver

FROM alpine:3.22
RUN adduser -D -H -u 1000 vtuner && mkdir /data && chown vtuner /data
COPY --from=build /out/vtunerd /out/fakereceiver /usr/local/bin/
USER vtuner
VOLUME /data
# -dev: the admin UI needs no DSM login. Allowed only because vtunerd listens on the
# container's loopback address; nginx in the same network namespace publishes it.
ENTRYPOINT ["vtunerd", "serve", "-dev", "-listen", "127.0.0.1:8580", "-data", "/data", "-search"]
