"""Export only local test credentials, without printing Supabase keys.

Usage: python3 scripts/local-test-env.py --github
For local use: python3 scripts/local-test-env.py --write (writes ignored web/.env.test.local).
"""
import base64
import json
import os
import pathlib
import subprocess
import sys

result = subprocess.run(["supabase", "status", "-o", "json"], check=True, capture_output=True, text=True)
status = json.loads(result.stdout)
if not status["API_URL"].startswith(("http://127.0.0.1:", "http://localhost:")):
    raise RuntimeError("Refusing to export a non-local project")
env = {
    "SUPABASE_URL": status["API_URL"],
    "SUPABASE_PUBLISHABLE_KEY": status["ANON_KEY"],
    "SUPABASE_SECRET_KEY": status["SERVICE_ROLE_KEY"],
    "BIODAYS_SITE_URL": "http://localhost:13000",
    "BIODAYS_CRON_SECRET": "local-test-cron-secret",
    "RESEND_WEBHOOK_SECRET": "whsec_" + base64.b64encode(b"local-webhook-secret-32-bytes-0000").decode(),
    "BIODAYS_ADMIN_EMAIL": "admin@biodays.test",
    "BIODAYS_ADMIN_GITHUB_IDS": "999000001",
}
if "--github" in sys.argv:
    with open(os.environ["GITHUB_ENV"], "a") as file:
        for key, value in env.items():
            file.write(f"{key}={value}\n")
elif "--write" in sys.argv:
    target = pathlib.Path(__file__).resolve().parents[1] / "web/.env.test.local"
    target.write_text("\n".join(f"{key}={json.dumps(value)}" for key, value in env.items()) + "\n")
    target.chmod(0o600)
    print("Wrote ignored web/.env.test.local for the isolated test project")
else:
    raise SystemExit("Choose --github or --write")
