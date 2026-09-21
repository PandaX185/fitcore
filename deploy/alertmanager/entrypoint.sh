#!/bin/sh
set -eu

# Render alertmanager.yml from the template, injecting the webhook URL only
# when ALERTMANAGER_WEBHOOK_URL is provided (common for a "no-op" setup).
WEBHOOK_URL="${ALERTMANAGER_WEBHOOK_URL:-}"
config=/etc/alertmanager/alertmanager.yml

{
  cat <<'YAML'
route:
  group_by: ["alertname"]
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: "default"

receivers:
  - name: "default"
YAML
  if [ -n "$WEBHOOK_URL" ]; then
    printf '    webhook_configs:\n      - url: "%s"\n        send_resolved: true\n' "$WEBHOOK_URL"
  fi
} > "$config"

exec /bin/alertmanager --config.file="$config" "$@"