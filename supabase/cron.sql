-- Run once in the production project's SQL editor after setting Vault secrets.
-- Vault names: biodays_site_url (e.g. https://biodays.example.com),
-- biodays_cron_secret (same value as Vercel BIODAYS_CRON_SECRET).
-- Do not enable this against preview deployments.
create extension if not exists pg_cron;
create extension if not exists pg_net with schema extensions;
do $$begin
 if exists(select 1 from cron.job where jobname='biodays-mail') then perform cron.unschedule('biodays-mail');end if;
 if exists(select 1 from cron.job where jobname='biodays-retention') then perform cron.unschedule('biodays-retention');end if;
end$$;
select cron.schedule('biodays-mail','* * * * *',$job$
 select net.http_post(
  url := (select decrypted_secret from vault.decrypted_secrets where name='biodays_site_url') || '/api/internal/mail/process',
  headers := jsonb_build_object('Content-Type','application/json','Authorization','Bearer ' || (select decrypted_secret from vault.decrypted_secrets where name='biodays_cron_secret')),
  body := '{}'::jsonb, timeout_milliseconds := 55000
 );
$job$);
select cron.schedule('biodays-retention','17 * * * *','select public.biodays_cleanup()');
