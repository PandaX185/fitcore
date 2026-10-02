#!/bin/sh
set -eu

# Render alertmanager.yml from the template, injecting the webhook URL only
# when ALERTMANAGER_WEBHOOK_URL is provided. With no webhook URL the default
# route falls back to the local "log" receiver (no integrations): alerts are
# not delivered anywhere but stay visible in the Alertmanager UI/API, and the
# fallback is logged here so `docker logs` shows delivery is local-only.
# An empty receiver would be invalid, but a named receiver with no
# integrations is a valid blackhole ("null receiver" pattern).
WEBHOOK_URL="${ALERTMANAGER_WEBHOOK_URL:-}"
config=/etc/alertmanager/alertmanager.yml

if [ -n "$WEBHOOK_URL" ]; then
  RECEIVER="default"
else
  RECEIVER="log"
  echo "ALERTMANAGER_WEBHOOK_URL is not set; using local log receiver (UI/API only)" >&2
fi

{
  cat <<YAML
route:
  group_by: ["alertname"]
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: "$RECEIVER"

receivers:
YAML
  if [ -n "$WEBHOOK_URL" ]; then
    cat <<YAML
  - name: "default"
    webhook_configs:
      - url: "$WEBHOOK_URL"
        send_resolved: true
YAML
  else
    cat <<'YAML'
  - name: "log"
YAML
  fi
} > "$config"

exec /bin/alertmanager --config.file="$config" "$@"