#!/bin/bash
# Patches the CoreDNS Corefile in the current kubectl context to resolve any
# "*.<INGRESS_HOST>" name to 127.0.0.1. This allows exposed OSCAR services
# using host-based routing (e.g. "myservice.<INGRESS_HOST>") to be reached
# without relying on the client's own DNS/hosts configuration.
#
# The domain can be customized via the INGRESS_HOST env var or as the first
# script argument (defaults to "localhost").
set -euo pipefail

NAMESPACE="kube-system"
CONFIGMAP="coredns"
DEPLOYMENT="coredns"
INGRESS_HOST="${1:-${INGRESS_HOST:-localhost}}"

if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl not found in PATH" >&2
    exit 1
fi

# Escape literal dots in the host so it can be embedded in the match regex.
ESCAPED_INGRESS_HOST=$(printf '%s' "$INGRESS_HOST" | sed 's/\./\\./g')

echo "[*] Fetching current CoreDNS Corefile from configmap/$CONFIGMAP -n $NAMESPACE ..."
CURRENT_COREFILE=$(kubectl get configmap "$CONFIGMAP" -n "$NAMESPACE" -o jsonpath='{.data.Corefile}')

if [ -z "$CURRENT_COREFILE" ]; then
    echo "Error: could not read Corefile from configmap/$CONFIGMAP -n $NAMESPACE" >&2
    exit 1
fi

if echo "$CURRENT_COREFILE" | grep -q "template IN A ${INGRESS_HOST} "; then
    echo "[*] Wildcard *.${INGRESS_HOST} template already present in CoreDNS Corefile. Nothing to do."
else
    echo "[*] Patching Corefile to resolve *.${INGRESS_HOST} to 127.0.0.1 ..."
    PATCH_FILEPATH=$(mktemp -t oscar-coredns-patch.XXXXXX)

    PATCH_BLOCK=$(cat <<EOF
    hosts {
        127.0.0.1 ${INGRESS_HOST}
        fallthrough
    }
    template IN A ${INGRESS_HOST} {
        match ^([^.]+)\.${ESCAPED_INGRESS_HOST}\.\$
        answer "{{ .Name }} 60 IN A 127.0.0.1"
        fallthrough
    }
EOF
    )

    # Insert the hosts + template block right before the closing brace of the
    # server block (the last "}" line of the Corefile). This is done with
    # plain sed/printf (instead of awk -v) because awk's -v assignment runs
    # its own escape-sequence processing on the value and would silently
    # strip the backslashes needed by the match regex above.
    LAST_LINE=$(printf '%s\n' "$CURRENT_COREFILE" | tail -n 1)
    if [[ ! "$LAST_LINE" =~ ^\}[[:space:]]*$ ]]; then
        echo "Error: unexpected Corefile format (last line is not a closing brace)" >&2
        exit 1
    fi

    printf '%s\n' "$CURRENT_COREFILE" | sed '$d' > "$PATCH_FILEPATH"
    printf '%s\n' "$PATCH_BLOCK" >> "$PATCH_FILEPATH"
    printf '%s\n' "$LAST_LINE" >> "$PATCH_FILEPATH"

    kubectl create configmap "$CONFIGMAP" -n "$NAMESPACE" \
        --from-file=Corefile="$PATCH_FILEPATH" \
        --dry-run=client -o yaml | kubectl apply -f -

    rm -f "$PATCH_FILEPATH"

    echo "[*] Restarting CoreDNS to apply the new configuration ..."
    kubectl -n "$NAMESPACE" rollout restart deployment/"$DEPLOYMENT"
    kubectl -n "$NAMESPACE" rollout status deployment/"$DEPLOYMENT" --timeout=120s

    echo "[OK] CoreDNS patched: *.${INGRESS_HOST} now resolves to 127.0.0.1"
fi

kubectl set env deployment/oscar -n oscar INGRESS_HOST="$INGRESS_HOST"