# Bootstrap image over upstream k0s for the temporary VyOS-hosted management cluster.
# OCI image tags use `-k0s.0` where GitHub release tags use `+k0s.0`.

# Version pins for the bootstrap control plane and startup-installed controllers.
ARG K0S_IMAGE=docker.io/k0sproject/k0s:v1.35.3-k0s.0
ARG CERT_MANAGER_VERSION=v1.20.2
ARG CAPI_OPERATOR_CHART_VERSION=0.26.0
ARG CUE_VERSION=v0.16.1
ARG CAPI_VERSION=v1.13.0
ARG CAPN_VERSION=v0.8.5
ARG TALOS_BOOTSTRAP_PROVIDER_VERSION=v0.6.12
ARG TALOS_CONTROL_PLANE_PROVIDER_VERSION=v0.5.13
ARG TINKERBELL_CHART_VERSION=v0.23.0
ARG HELM_VERSION=v4.0.4

# Build the chart artifacts and rendered provider manifests ahead of time so the
# runtime image can come up without extra fetch or template steps.
FROM docker.io/alpine:3.22 AS chart-builder

ARG TARGETARCH
ARG CERT_MANAGER_VERSION
ARG CAPI_OPERATOR_CHART_VERSION
ARG CUE_VERSION
ARG CAPI_VERSION
ARG CAPN_VERSION
ARG TALOS_BOOTSTRAP_PROVIDER_VERSION
ARG TALOS_CONTROL_PLANE_PROVIDER_VERSION
ARG TINKERBELL_CHART_VERSION
ARG HELM_VERSION

RUN apk add --no-cache \
    curl=8.14.1-r2 \
    gzip=1.14-r1 \
    tar=1.35-r3

WORKDIR /work

# Fetch the small toolchain needed to package charts and render provider CRs.
RUN curl -fsSL -o /tmp/helm.tgz "https://get.helm.sh/helm-${HELM_VERSION}-linux-${TARGETARCH}.tar.gz" \
    && tar -xzf /tmp/helm.tgz --strip-components=1 -C /usr/local/bin "linux-${TARGETARCH}/helm" \
    && rm -f /tmp/helm.tgz

RUN curl -fsSL -o /tmp/cue.tgz "https://github.com/cue-lang/cue/releases/download/${CUE_VERSION}/cue_${CUE_VERSION}_linux_${TARGETARCH}.tar.gz" \
    && tar -xzf /tmp/cue.tgz -C /usr/local/bin cue \
    && rm -f /tmp/cue.tgz

RUN mkdir -p /out

COPY bootstrap/k0s/manifests/providers/providers.cue /tmp/providers.cue

# Pull the startup charts into fixed local artifact names for the runtime image.
RUN helm pull cert-manager \
        --repo https://charts.jetstack.io \
        --version "${CERT_MANAGER_VERSION}" \
        --destination /out \
    && mv "/out/cert-manager-${CERT_MANAGER_VERSION}.tgz" /out/cert-manager.tgz

RUN helm repo add capi-operator https://kubernetes-sigs.github.io/cluster-api-operator >/dev/null \
    && helm pull capi-operator/cluster-api-operator \
        --version "${CAPI_OPERATOR_CHART_VERSION}" \
        --destination /out \
    && mv "/out/cluster-api-operator-${CAPI_OPERATOR_CHART_VERSION}.tgz" /out/cluster-api-operator.tgz

RUN helm pull oci://ghcr.io/tinkerbell/charts/tinkerbell \
        --version "${TINKERBELL_CHART_VERSION}" \
        --destination /out \
    && mv "/out/tinkerbell-${TINKERBELL_CHART_VERSION}.tgz" /out/tinkerbell.tgz

# Render the provider CRs once during the build; the runtime handoff script will
# only stage them into the watched manifest directory after the operator is ready.
RUN cue export /tmp/providers.cue \
        -e output \
        --out yaml \
        -t capi_version="${CAPI_VERSION}" \
        -t capn_version="${CAPN_VERSION}" \
        -t talos_bootstrap_provider_version="${TALOS_BOOTSTRAP_PROVIDER_VERSION}" \
        -t talos_control_plane_provider_version="${TALOS_CONTROL_PLANE_PROVIDER_VERSION}" \
        >/out/providers.yaml

# The runtime image only needs the CUE sources, the CUE CLI, pre-fetched
# charts, and the delayed provider-manifest handoff helper.
FROM ${K0S_IMAGE}

COPY bootstrap/k0s/k0s.cue /opt/bootstrap/k0s.cue
COPY bootstrap/k0s/manifests/providers/namespaces.yaml /var/lib/k0s/manifests/providers/00-namespaces.yaml
COPY bootstrap/k0s/bootstrap-k0s.sh /usr/local/bin/bootstrap-k0s.sh
COPY --from=chart-builder /usr/local/bin/cue /usr/local/bin/cue
COPY --from=chart-builder /out/cert-manager.tgz /opt/bootstrap/charts/cert-manager.tgz
COPY --from=chart-builder /out/cluster-api-operator.tgz /opt/bootstrap/charts/cluster-api-operator.tgz
COPY --from=chart-builder /out/tinkerbell.tgz /opt/bootstrap/charts/tinkerbell.tgz
COPY --from=chart-builder /out/providers.yaml /opt/bootstrap/providers/providers.yaml

ENV K0S_ENTRYPOINT_ROLE=controller+worker

# Namespace manifests are always present; provider CRs are staged later by the
# helper once the operator CRDs and webhook are available.
RUN mkdir -p /etc/k0s /opt/bootstrap/charts \
    && chmod 0755 /usr/local/bin/bootstrap-k0s.sh

CMD ["/usr/local/bin/bootstrap-k0s.sh"]
