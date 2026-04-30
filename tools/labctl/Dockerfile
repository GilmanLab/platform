FROM cgr.dev/chainguard/static

ARG TARGETPLATFORM

COPY ${TARGETPLATFORM}/labctl /usr/local/bin/labctl

ENTRYPOINT ["/usr/local/bin/labctl"]
