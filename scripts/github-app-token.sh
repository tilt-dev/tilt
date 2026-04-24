#!/bin/bash
#
# Exchanges github app private key for an installation access token.
#
# Usage:
#   GITHUB_CLIENT_ID=... GITHUB_PRIVATE_KEY_BASE64=... GITHUB_INSTALLATION_ID=... ./github-app-token.sh

set -euo pipefail

# Adapted from
# https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app
# https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app
# Decode the base64-encoded private key
pem=$(echo "$GITHUB_PRIVATE_KEY_BASE64" | base64 --decode)

now=$(date +%s)
iat=$((${now} - 60)) # Issues 60 seconds in the past
exp=$((${now} + 600)) # Expires 10 minutes in the future

b64enc() { openssl base64 | tr -d '=' | tr '/+' '_-' | tr -d '\n'; }

header_json='{    "typ":"JWT",
    "alg":"RS256"
}'
# Header encode
header=$( echo -n "${header_json}" | b64enc )

payload_json="{    \"iat\":${iat},
    \"exp\":${exp},
    \"iss\":\"${GITHUB_CLIENT_ID}\"
}"
# Payload encode
payload=$( echo -n "${payload_json}" | b64enc )

# Signature
header_payload="${header}"."${payload}"
signature=$(
    openssl dgst -sha256 -sign <(echo -n "${pem}") \
    <(echo -n "${header_payload}") | b64enc
)

# Create JWT
JWT="${header_payload}"."${signature}"
token=$(curl -sSL --request POST \
--url "https://api.github.com/app/installations/$GITHUB_INSTALLATION_ID/access_tokens" \
--header "Accept: application/vnd.github+json" \
--header "Authorization: Bearer $JWT" \
--header "X-GitHub-Api-Version: 2026-03-10" | jq -r '.token')
if [[ "$token" == "" ]]; then
    echo "Failed to get token" >&2
    exit 1
fi
echo "$token"
