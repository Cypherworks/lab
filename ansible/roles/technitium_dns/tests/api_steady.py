"""Fails if a web server drops a connection while it's being watched. The
tests use it to catch Technitium's restart after a web service setting, which
lasts under 40 ms. Every check reuses one kept-alive connection, as a restart
has to close it whatever the timing.

Usage: api_steady.py PORT SECONDS
"""
import http.client
import sys
import time

port, seconds = int(sys.argv[1]), float(sys.argv[2])
end = time.time() + seconds
checks = 0
connection = http.client.HTTPConnection("127.0.0.1", port, timeout=2)
while time.time() < end:
    try:
        connection.request("GET", "/")
        connection.getresponse().read()
    except (OSError, http.client.HTTPException) as error:
        sys.exit(f"dropped after {checks} checks: {error!r}")
    checks += 1
    time.sleep(0.05)
print(f"steady over {checks} checks")
