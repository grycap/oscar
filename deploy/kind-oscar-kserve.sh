#!/bin/bash
set -euo pipefail

if kubectl get namespace cert-manager >/dev/null 2>&1 && \
  kubectl get deployment -n cert-manager cert-manager cert-manager-cainjector cert-manager-webhook >/dev/null 2>&1; then
  echo "cert-manager already installed, skipping install"
else
  kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.19.1/cert-manager.yaml
fi

kubectl wait --namespace cert-manager \
  --for=condition=Available deployment \
  cert-manager cert-manager-cainjector cert-manager-webhook \
  --timeout=300s

helm upgrade --install kserve-crd oci://ghcr.io/kserve/charts/kserve-crd \
  --version v0.19.0-rc0 \
  --namespace kserve \
  --create-namespace \
  --wait

helm upgrade --install kserve-resources oci://ghcr.io/kserve/charts/kserve-resources \
  --version v0.19.0-rc0 \
  --namespace kserve \
  --set kserve.controller.deploymentMode=Standard \
  --set kserve.controller.gateway.disableIngressCreation=true \
  --set kserve.controller.gateway.disableIstioVirtualHost=true \
  --set 'kserve.controller.gateway.domainTemplate=\{\{ .Name \}\}.\{\{ .IngressDomain \}\}' \
  --set kserve.controller.gateway.ingressGateway.enableGatewayApi=true \
  --set kserve.controller.gateway.ingressGateway.kserveGateway="traefik/traefik-gateway" \
  --set kserve.controller.gateway.ingressGateway.createGateway=false \
  --wait

helm upgrade --install kserve-llmisvc-crd oci://ghcr.io/kserve/charts/kserve-llmisvc-crd \
  --version v0.19.0-rc0 \
  --namespace kserve \
  --create-namespace \
  --wait

helm upgrade --install kserve-llmisvc-resources oci://ghcr.io/kserve/charts/kserve-llmisvc-resources \
  --version v0.19.0-rc0 \
  --create-namespace \
  --namespace kserve \
  --set kserve.createSharedResources=false \
  --wait

helm upgrade --install kserve-runtime-configs oci://ghcr.io/kserve/charts/kserve-runtime-configs \
  --version v0.19.0-rc0 \
  --namespace kserve \
  --set kserve.llmisvcConfigs.enabled=true \
  --set kserve.servingruntime.enabled=true \
  --wait 

kubectl set env deployment/oscar -n oscar KSERVE_ENABLE=true
kubectl set env deployment/oscar -n oscar EXPOSED_SERVICES_ROUTE_KIND=httproute 
