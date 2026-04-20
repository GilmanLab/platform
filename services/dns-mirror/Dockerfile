FROM golang:1.26.2-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETARCH
RUN if [ -z "${TARGETARCH}" ]; then TARGETARCH="$(go env GOARCH)"; fi && \
    CGO_ENABLED=0 GOOS=linux GOARCH="${TARGETARCH}" go build -trimpath -o /out/dns-mirror ./cmd/dns-mirror

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

COPY --from=build /out/dns-mirror /usr/local/bin/dns-mirror

ENTRYPOINT ["/usr/local/bin/dns-mirror"]
