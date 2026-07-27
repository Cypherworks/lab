#!/usr/bin/env bash
# headscale-oidc-watchdog — restart headscale to load OIDC when its provider is
# reachable but the running instance is serving CLI-only enrolment.
#
# WHY: headscale configures its OIDC provider only at startup. With
# only_start_if_oidc_is_available=false it starts even when the issuer (Authentik,
# auth.cw) is unreachable, logs "failed to set up OIDC provider, falling back to CLI
# based authentication", and serves manual `headscale nodes register` enrolment. A
# restart with the issuer reachable makes it load OIDC; this watchdog performs that
# restart so enrolment needs no human.
#
# Run from a systemd timer as root. Configured via the environment:
#   HEADSCALE_OIDC_DISCOVERY_URL     the issuer's .well-known/openid-configuration
#   HEADSCALE_OIDC_FAILURE_MARKER    the log line headscale writes on OIDC failure
#   HEADSCALE_OIDC_WATCHDOG_TIMEOUT  curl timeout in seconds (default 5)
#
# Exit 0 always: a probe failure is not an error — headscale must keep serving
# existing nodes while the issuer is down.
set -uo pipefail

log() { echo "headscale-oidc-watchdog: $*"; }

# Pure decision, kept side-effect-free so it is unit-testable: restart only when
# the issuer is reachable AND headscale started in the degraded (OIDC-less) state.
decide() { # <reachable:yes|no> <degraded:yes|no> -> "restart"|"noop"
  if [ "${1:-}" = "yes" ] && [ "${2:-}" = "yes" ]; then
    echo restart
  else
    echo noop
  fi
}

# Is the OIDC discovery document being served? (Authentik up and reachable.)
oidc_reachable() {
  local url="${HEADSCALE_OIDC_DISCOVERY_URL:?HEADSCALE_OIDC_DISCOVERY_URL not set}"
  local timeout="${HEADSCALE_OIDC_WATCHDOG_TIMEOUT:-5}" code
  code=$(curl -sS -o /dev/null -w '%{http_code}' --max-time "$timeout" "$url" 2>/dev/null) || return 1
  [ "$code" = "200" ]
}

# Did the CURRENT headscale run fail to set up OIDC? Scoped to this systemd
# invocation, so a restart clears the signal and the watchdog settles.
headscale_started_degraded() {
  local marker="${HEADSCALE_OIDC_FAILURE_MARKER:-failed to set up OIDC provider}"
  local inv
  inv=$(systemctl show headscale -p InvocationID --value 2>/dev/null) || return 1
  [ -n "$inv" ] || return 1
  journalctl "_SYSTEMD_INVOCATION_ID=$inv" --no-pager 2>/dev/null | grep -qF "$marker"
}

main() {
  local reachable=no degraded=no
  oidc_reachable && reachable=yes
  headscale_started_degraded && degraded=yes

  if [ "$(decide "$reachable" "$degraded")" = restart ]; then
    log "OIDC issuer reachable but headscale is running OIDC-degraded — restarting headscale"
    systemctl restart headscale
  else
    log "no action (issuer reachable=$reachable, headscale degraded=$degraded)"
  fi
}

# Run only when executed, not when sourced by the test.
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
  main "$@"
fi
