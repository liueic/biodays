"""Verify public reading survives a failed Supabase connection after a build."""
import os
import pathlib
import socket
import subprocess
import time
import urllib.error
import urllib.request

root = pathlib.Path(__file__).resolve().parents[1]
with socket.socket() as sock:
    sock.bind(("127.0.0.1", 0))
    port = sock.getsockname()[1]
origin = f"http://127.0.0.1:{port}"
env = {**os.environ, "BIODAYS_SITE_URL": origin, "SUPABASE_URL": "http://127.0.0.1:1", "SUPABASE_SECRET_KEY": "offline-test-key", "BIODAYS_DISABLE_ANALYTICS": "true"}
server = subprocess.Popen(["node", "node_modules/next/dist/bin/next", "start", "--port", str(port)], cwd=root / "web", env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
def status(path):
    try:
        with urllib.request.urlopen(origin + path, timeout=10) as response:
            return response.status
    except urllib.error.HTTPError as error:
        return error.code
try:
    for attempt in range(60):
        try:
            if status("/healthz") == 200:
                break
        except OSError:
            time.sleep(0.25)
    else:
        raise RuntimeError("Next server did not start")
    for path in ["/", "/archive", "/story/Q1-01", "/api/v1/stories", "/sitemap.xml"]:
        assert status(path) == 200, path
    assert status("/api/v1/feedback/options") == 503
    assert status("/readyz") == 503
    print("Public content remains available while Supabase is offline")
finally:
    server.terminate()
    try:
        server.wait(timeout=10)
    except subprocess.TimeoutExpired:
        server.kill()
        server.wait()
