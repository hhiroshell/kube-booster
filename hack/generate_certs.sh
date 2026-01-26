#!/usr/bin/env bash

# Script to generate self-signed certificates for kube-booster webhook
# This is for local testing only. For production, use cert-manager.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="${SCRIPT_DIR}/../certs"
NAMESPACE="kube-system"
SERVICE_NAME="kube-booster-webhook-service"
SECRET_NAME="kube-booster-webhook-cert"

# Create cert directory
mkdir -p "${CERT_DIR}"

echo "Generating certificates for kube-booster webhook..."

# Generate CA private key
openssl genrsa -out "${CERT_DIR}/ca.key" 2048

# Generate CA certificate
openssl req -x509 -new -nodes -key "${CERT_DIR}/ca.key" \
  -subj "/CN=kube-booster-ca" \
  -days 3650 \
  -out "${CERT_DIR}/ca.crt"

# Generate server private key
openssl genrsa -out "${CERT_DIR}/tls.key" 2048

# Create certificate signing request (CSR)
cat > "${CERT_DIR}/csr.conf" <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name

[req_distinguished_name]

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${SERVICE_NAME}
DNS.2 = ${SERVICE_NAME}.${NAMESPACE}
DNS.3 = ${SERVICE_NAME}.${NAMESPACE}.svc
DNS.4 = ${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local
EOF

# Generate CSR
openssl req -new -key "${CERT_DIR}/tls.key" \
  -subj "/CN=${SERVICE_NAME}.${NAMESPACE}.svc" \
  -out "${CERT_DIR}/server.csr" \
  -config "${CERT_DIR}/csr.conf"

# Sign the certificate with the CA
openssl x509 -req -in "${CERT_DIR}/server.csr" \
  -CA "${CERT_DIR}/ca.crt" \
  -CAkey "${CERT_DIR}/ca.key" \
  -CAcreateserial \
  -out "${CERT_DIR}/tls.crt" \
  -days 3650 \
  -extensions v3_req \
  -extfile "${CERT_DIR}/csr.conf"

echo "Certificates generated in ${CERT_DIR}"

# Create Kubernetes secret
echo ""
echo "Creating Kubernetes secret..."
kubectl create secret generic "${SECRET_NAME}" \
  --from-file=tls.crt="${CERT_DIR}/tls.crt" \
  --from-file=tls.key="${CERT_DIR}/tls.key" \
  --namespace="${NAMESPACE}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Secret ${SECRET_NAME} created in namespace ${NAMESPACE}"

# Print CA bundle for webhook configuration
echo ""
echo "CA Bundle (base64 encoded) for webhook configuration:"
echo "========================================================"
CA_BUNDLE=$(base64 < "${CERT_DIR}/ca.crt" | tr -d '\n')
echo "${CA_BUNDLE}"
echo ""
echo "Update config/webhook/mutating_webhook.yaml with this CA bundle:"
echo "Replace \${CA_BUNDLE} with the above value"
echo ""

# Optionally update the webhook configuration automatically
if command -v yq &> /dev/null; then
  echo "Updating mutating_webhook.yaml with CA bundle..."
  WEBHOOK_FILE="${SCRIPT_DIR}/../config/webhook/mutating_webhook.yaml"
  # Use sed for simpler replacement
  sed -i.bak "s|\${CA_BUNDLE}|${CA_BUNDLE}|g" "${WEBHOOK_FILE}"
  echo "Updated ${WEBHOOK_FILE}"
  echo "(Original backed up as ${WEBHOOK_FILE}.bak)"
else
  echo "Note: Install 'yq' to automatically update the webhook configuration"
fi

echo ""
echo "Certificate generation complete!"
