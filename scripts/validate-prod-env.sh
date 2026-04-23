#!/usr/bin/env sh
set -eu

ENV_FILE="${1:-.env.production}"

if [ ! -f "$ENV_FILE" ]; then
  echo "Missing env file: $ENV_FILE"
  exit 1
fi

required_vars="APP_CLIENT_ID APP_CLIENT_SECRET OAUTH_CALLBACK_URL CORS_ALLOWED_ORIGINS APP_ID APP_PRIVATE_KEY GOOGLE_API_KEY"

missing=0
for var in $required_vars; do
  if ! grep -q "^${var}=" "$ENV_FILE"; then
    echo "Missing key: ${var}"
    missing=1
    continue
  fi

  value=$(grep "^${var}=" "$ENV_FILE" | head -n1 | cut -d '=' -f2-)
  if [ -z "$value" ]; then
    echo "Empty value: ${var}"
    missing=1
  fi
done

if [ "$missing" -ne 0 ]; then
  echo "Production env validation failed."
  exit 1
fi

echo "Production env validation passed."
