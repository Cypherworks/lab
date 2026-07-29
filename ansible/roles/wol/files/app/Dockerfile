# Build the wol binary, then ship it on a minimal base. Managed by the wol role.
FROM golang:1.25.12-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /wol .

FROM alpine:3.20
# libcap grants the binary CAP_NET_RAW so the status ping works as a non-root user;
# the container itself runs with cap_drop ALL + cap_add NET_RAW.
RUN apk add --no-cache libcap \
 && addgroup -S wol \
 && adduser -S -G wol wol
COPY --from=build /wol /usr/local/bin/wol
RUN setcap cap_net_raw+ep /usr/local/bin/wol
USER wol
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -q -O - http://127.0.0.1:8090/healthz >/dev/null 2>&1 || exit 1
ENTRYPOINT ["/usr/local/bin/wol"]
