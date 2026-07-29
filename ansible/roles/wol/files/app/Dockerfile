# Build the wol binary, then ship it on a minimal base. Managed by the wol role.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /wol .

FROM alpine:3.20
COPY --from=build /wol /usr/local/bin/wol
ENTRYPOINT ["/usr/local/bin/wol"]
