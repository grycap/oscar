
#!/usr/bin/env bash
set -euo pipefail

usage() {
	cat <<'EOF'
Usage: kind-add-kserve.sh [--basic | --more-runtimes | --delete | --help]

Options:
	--basic           Install KServe core components in namespace kserve-test.
	--more-runtimes   Install extra KServe runtimes in namespace kserve-test.
	--delete          Remove KServe installation from namespace kserve-test.
	--help            Show this help message.
EOF
}

require_arg() {
	if [[ $# -eq 0 ]]; then
		echo "Error: an option is required." >&2
		usage
		exit 1
	fi
}

install_basic() {
	kubectl create namespace kserve-test > /dev/null || true
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.17.4/cert-manager.yaml
	wait_for_cert_manager
	helm install kserve-crd oci://ghcr.io/kserve/charts/kserve-crd --version v0.16.0 || true
	helm install -n kserve-test kserve oci://ghcr.io/kserve/charts/kserve --version v0.16.0
	kubectl patch configmap/inferenceservice-config -n kserve-test --type=strategic -p '{"data": {"ingress": "{ \"enableGatewayApi\": false, \"kserveIngressGateway\": \"kserve/kserve-ingress-gateway\", \"ingressGateway\": \"knative-serving/knative-ingress-gateway\", \"localGateway\": \"knative-serving/knative-local-gateway\", \"localGatewayService\": \"knative-local-gateway.istio-system.svc.cluster.local\", \"ingressDomain\": \"example.com\", \"additionalIngressDomains\": [\"additional-example.com\", \"additional-example-1.com\"], \"ingressClassName\": \"istio\", \"domainTemplate\": \"-.\", \"urlScheme\": \"http\", \"disableIstioVirtualHost\": true, \"disableIngressCreation\": false }"}}'
	kubectl rollout restart deployment kserve-controller-manager -n kserve-test
}

wait_for_cert_manager() {
	kubectl -n cert-manager wait --for=condition=Available deployment/cert-manager deployment/cert-manager-cainjector deployment/cert-manager-webhook --timeout=180s
	kubectl wait --for=condition=Established crd/certificaterequests.cert-manager.io crd/certificates.cert-manager.io crd/clusterissuers.cert-manager.io crd/issuers.cert-manager.io --timeout=120s
	ensure_webhook_ca_bundle
}

ensure_webhook_ca_bundle() {
	for _ in $(seq 1 30); do
		if kubectl get validatingwebhookconfiguration cert-manager-webhook -o jsonpath="{.webhooks[*].clientConfig.caBundle}" | grep -qE '[A-Za-z0-9+/]{10,}'; then
			return 0
		fi
		sleep 5
	done
	echo "Error: cert-manager webhook CA bundle was not injected in time." >&2
	exit 1
}

install_more_runtimes() {
	kubectl apply -k https://github.com/kserve/kserve/config/runtimes -n kserve-test --server-side
  kubectl rollout restart deployment kserve-controller-manager -n kserve-test
}

delete_installation() {
	helm uninstall kserve -n kserve-test
	helm uninstall kserve-crd
	kubectl delete -f https://github.com/cert-manager/cert-manager/releases/download/v1.17.4/cert-manager.yaml
	kubectl delete namespace kserve-test
}

require_arg "$@"

case "$1" in
	--basic)
		install_basic
		;;
	--more-runtimes)
		install_more_runtimes
		;;
	--delete)
		delete_installation
		;;
	--help)
		usage
		;;
	*)
		echo "Error: unknown option '$1'" >&2
		usage
		exit 1
		;;
esac