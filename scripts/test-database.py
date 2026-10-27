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
        result = subprocess.run(["docker", "exec", container, "pg_isready", "-U", "postgres"], capture_output=True)
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
    for migration in migrations[1:]:
        sql(migration.read_text())
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
