"""Run database regressions in an isolated, disposable PostgreSQL container."""
import os
import pathlib
import subprocess
import time
import uuid

root = pathlib.Path(__file__).resolve().parents[1]
container = "biodays-test-" + uuid.uuid4().hex[:10]
def run(*args, **kwargs):
    return subprocess.run(args, check=True, text=True, **kwargs)
try:
    run("docker", "run", "--rm", "-d", "--name", container,
        "-e", "POSTGRES_PASSWORD=test-only", "postgres:17", stdout=subprocess.DEVNULL)
    for attempt in range(60):
        # The image's temporary initialization server accepts Unix sockets only.
        # Wait for the final TCP server so SQL cannot race its restart.
        result = subprocess.run(["docker", "exec", container, "pg_isready", "-h", "127.0.0.1", "-U", "postgres"], capture_output=True)
        if result.returncode == 0:
            break
        time.sleep(0.5)
    else:
        raise RuntimeError("Test PostgreSQL failed to start")
    def sql(source):
        return run("docker", "exec", "-i", container, "psql", "-U", "postgres", "-v", "ON_ERROR_STOP=1", input=source, capture_output=True).stdout
    sql("create role anon; create role authenticated; create role service_role bypassrls; create schema auth; create table auth.users(id uuid primary key);")
    migrations = sorted((root / "supabase/migrations").glob("*.sql"))
    sql(migrations[0].read_text())
    sql("insert into biodays_feedback(id,request_id,request_body_hash,kind,title,content,spam_confidence) values('pre-jev','pre-jev','legacy','issue','旧反馈','这是迁移前已经保存的一条旧反馈内容。',0.95);")
    legacy_settings = '{"subscriptions_enabled":false,"resend":{"enabled":false,"api_key":"v1:old-resend","from":"old@example.com","notify_to":"old-admin@example.com"},"turnstile":{"enabled":false,"secret_key":"v1:old-turnstile","site_key":"old-site","hostnames":["old.test"]},"ai_spam":{"enabled":true,"model":"typesafe-ai/jev-1.14.0","daily_limit":123,"api_key":"v1:old-gateway","account_id":"old-account","api_token":"v1:old-cloudflare","style":"decision"}}'
    sql("insert into biodays_settings(id,document) values(true,'" + legacy_settings + "');")
    for migration in migrations[1:]:
        sql(migration.read_text())
    assert sql("select revision=1 and document->>'subscriptions_enabled'='false' and document#>>'{ai_spam,daily_limit}'='123' and document#>>'{ai_spam,enabled}'='false' and document::text !~ 'api_key|api_token|account_id|secret_key|notify_to|hostnames|old-site|old@example' from biodays_settings;").splitlines()[2].strip() == "t"
    # Cleanup is idempotent and never copies ciphertext into audit records.
    sql(migrations[-1].read_text())
    assert sql("select revision=1 from biodays_settings;").splitlines()[2].strip() == "t"
    assert sql("select count(*)=1 and bool_and(changes !~ 'v1:|old-resend|old-gateway|old-turnstile') from biodays_settings_audit;").splitlines()[2].strip() == "t"
    sql("delete from biodays_settings_audit; delete from biodays_settings;")
    assert sql("select spam_probability is null and spam_confidence=0.95::real from biodays_feedback where id='pre-jev';").splitlines()[2].strip() == "t"
    sql("delete from biodays_feedback where id='pre-jev';")
    sql((root / "supabase/tests/regression.sql").read_text())
    # Concurrent saves from different instances must not both succeed.
    sql("insert into biodays_settings(id,document) values(true,'{}');")
    from concurrent.futures import ThreadPoolExecutor
    def save(actor):
        return subprocess.run(["docker", "exec", "-i", container, "psql", "-U", "postgres", "-v", "ON_ERROR_STOP=1"], input=f"select biodays_save_settings(0,'{{}}','{actor}','{{}}');", capture_output=True, text=True).returncode
    with ThreadPoolExecutor(max_workers=2) as executor:
        results = list(executor.map(save, ["admin-a", "admin-b"]))
    assert sorted(results) == [0, 3], results
    assert sql("select revision from biodays_settings;").splitlines()[2].strip() == "1"
    print("Database upgrade, regressions and concurrent settings CAS passed")
finally:
    subprocess.run(["docker", "stop", container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
