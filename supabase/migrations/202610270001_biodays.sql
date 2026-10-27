-- All business data is private. Only the Vercel server's service_role may use it.
create table public.biodays_admins (user_id uuid primary key references auth.users(id) on delete cascade);
create table public.biodays_settings (id boolean primary key default true check(id), revision integer not null default 0, document jsonb not null, updated_at timestamptz not null default now(), updated_by text not null default 'bootstrap');
create table public.biodays_settings_audit (id bigint generated always as identity primary key, revision integer not null, actor text not null, changes text not null, created_at timestamptz not null default now());
create table public.biodays_secrets (id boolean primary key default true check(id), abuse_key text not null default replace(gen_random_uuid()::text || gen_random_uuid()::text,'-',''));
insert into public.biodays_secrets(id) values(true);
create table public.biodays_feedback (
 id text primary key, request_id text not null unique, request_body_hash text not null,
 kind text not null check(kind in ('topic','resource','feature','issue')),
 title text not null check(char_length(title) between 1 and 80), content text not null check(char_length(content) between 10 and 5000),
 content_hash text not null default '', email_domain text not null default '', source_url text not null default '', story_id text not null default '', nickname text not null default '', email text not null default '',
 status text not null default 'pending' check(status in ('pending','reviewing','accepted','completed','rejected','spam')),
 internal_note text not null default '', public_reply text not null default '', version integer not null default 1,
 subscription_status text not null default 'none' check(subscription_status in ('none','pending_confirmation','confirmed','unsubscribed')),
 confirmation_email text not null default 'none', consent_version text not null default '', consented_at timestamptz, confirmed_at timestamptz, unsubscribed_at timestamptz,
 spam_score integer not null default 0, spam_verdict text not null default 'allow', spam_signals jsonb not null default '[]', spam_source text not null default 'heuristic', spam_model text not null default '', spam_confidence real not null default 0,
 verification_method text not null default 'none', verified_at timestamptz,
 created_at timestamptz not null default now(), updated_at timestamptz not null default now(), closed_at timestamptz
);
create index on public.biodays_feedback(status, created_at desc);
create index on public.biodays_feedback(kind, created_at desc);
create index on public.biodays_feedback(content_hash,created_at);
create index on public.biodays_feedback(email_domain,created_at);
create index on public.biodays_feedback(closed_at);
create table public.biodays_events (
 id bigint generated always as identity primary key, feedback_id text not null references public.biodays_feedback(id) on delete cascade,
 seq integer not null, from_status text not null, to_status text not null, internal_note text not null default '', public_reply text not null default '', actor text not null,
 created_at timestamptz not null default now(), unique(feedback_id,seq)
);
create table public.biodays_tokens (
 token_hash text primary key, feedback_id text not null references public.biodays_feedback(id) on delete cascade, purpose text not null check(purpose in ('confirm','unsubscribe')),
 expires_at timestamptz not null, consumed_at timestamptz, created_at timestamptz not null default now()
);
create index on public.biodays_tokens(feedback_id,purpose);
create index on public.biodays_tokens(expires_at);
create table public.biodays_outbox (
 id text primary key, feedback_id text references public.biodays_feedback(id) on delete cascade, type text not null, event_key text not null unique,
 recipient text not null, payload jsonb, status text not null default 'pending' check(status in ('pending','sending','sent','failed','unknown','cancelled')),
 attempts integer not null default 0, next_attempt_at timestamptz not null default now(), lease_until timestamptz, lease_token uuid,
 first_attempt_at timestamptz, last_error text not null default '', provider_message_id text unique, provider_key_hash text not null default '', provider_accepted_at timestamptz,
 delivery_status text not null default '', delivery_rank integer not null default 0,
 created_at timestamptz not null default now(), updated_at timestamptz not null default now()
);
create index on public.biodays_outbox(status,next_attempt_at);
create index on public.biodays_outbox(feedback_id);
create table public.biodays_webhooks (id text primary key, message_id text not null, type text not null, created_at timestamptz not null default now());
create table public.biodays_suppressions (email text primary key, reason text not null, created_at timestamptz not null default now());
create table public.biodays_abuse (key_hash text not null, window_start timestamptz not null, hits integer not null default 1, expires_at timestamptz not null, primary key(key_hash,window_start));
create table public.biodays_deletions (id bigint generated always as identity primary key, feedback_id text not null, actor text not null, created_at timestamptz not null default now());

create function public.biodays_enqueue(p_mail jsonb) returns void language plpgsql set search_path = public as $$
begin
 insert into biodays_outbox(id, feedback_id, type, event_key, recipient, payload)
 select m->>'id', nullif(m->>'feedback_id',''), m->>'type', m->>'event_key', m->>'recipient', m->'payload'
 from jsonb_array_elements(coalesce(p_mail,'[]')) m on conflict(event_key) do nothing;
end $$;
create function public.biodays_add_tokens(p_tokens jsonb) returns void language sql set search_path = public as $$
 insert into biodays_tokens(token_hash,feedback_id,purpose,expires_at)
 select t->>'token_hash',t->>'feedback_id',t->>'purpose',(t->>'expires_at')::timestamptz from jsonb_array_elements(coalesce(p_tokens,'[]')) t;
$$;

create function public.biodays_submit(p_feedback jsonb,p_request_id text,p_hash text,p_tokens jsonb,p_mail jsonb) returns jsonb language plpgsql set search_path = public as $$
declare f biodays_feedback;
begin
 perform pg_advisory_xact_lock(hashtextextended(p_request_id,0));
 select * into f from biodays_feedback where request_id=p_request_id;
 if found then
  if f.request_body_hash<>p_hash then raise exception 'IDEMPOTENCY_CONFLICT'; end if;
  return to_jsonb(f)||jsonb_build_object('_created',false);
 end if;
 insert into biodays_feedback(id,request_id,request_body_hash,kind,title,content,content_hash,email_domain,source_url,story_id,nickname,email,status,subscription_status,confirmation_email,consent_version,consented_at,spam_score,spam_verdict,spam_signals,spam_source,spam_model,spam_confidence,verification_method,verified_at)
 values(p_feedback->>'id',p_request_id,p_hash,p_feedback->>'kind',p_feedback->>'title',p_feedback->>'content',p_feedback->>'content_hash',p_feedback->>'email_domain',p_feedback->>'source_url',p_feedback->>'story_id',p_feedback->>'nickname',p_feedback->>'email',p_feedback->>'status',p_feedback->>'subscription_status',p_feedback->>'confirmation_email',p_feedback->>'consent_version',(p_feedback->>'consented_at')::timestamptz,(p_feedback->>'spam_score')::integer,p_feedback->>'spam_verdict',p_feedback->'spam_signals',p_feedback->>'spam_source',p_feedback->>'spam_model',(p_feedback->>'spam_confidence')::real,p_feedback->>'verification_method',(p_feedback->>'verified_at')::timestamptz) returning * into f;
 if f.status='spam' then update biodays_feedback set closed_at=now() where id=f.id; end if;
 insert into biodays_events(feedback_id,seq,from_status,to_status,actor) values(f.id,1,'',f.status,'visitor');
 perform biodays_add_tokens(p_tokens); perform biodays_enqueue(p_mail);
 return to_jsonb(f)||jsonb_build_object('_created',true);
end $$;

create function public.biodays_save_settings(p_base integer,p_document jsonb,p_actor text,p_changes text) returns jsonb language plpgsql set search_path = public as $$
declare r biodays_settings;
begin
 update biodays_settings set document=p_document,revision=revision+1,updated_at=now(),updated_by=p_actor where id and revision=p_base returning * into r;
 if not found then raise exception 'VERSION_CONFLICT'; end if;
 insert into biodays_settings_audit(revision,actor,changes) values(r.revision,p_actor,p_changes);
 return to_jsonb(r);
end $$;

create function public.biodays_rate(p_buckets jsonb) returns boolean language plpgsql set search_path = public as $$
declare b jsonb; n integer;
begin
 -- Lock in stable order to avoid cross-bucket deadlocks. Counters survive rejections.
 for b in select value from jsonb_array_elements(p_buckets) order by value->>'key' loop
  insert into biodays_abuse(key_hash,window_start,expires_at) values(b->>'key',(b->>'start')::timestamptz,(b->>'expires')::timestamptz)
  on conflict(key_hash,window_start) do update set hits=biodays_abuse.hits+1 returning hits into n;
  if n>(b->>'limit')::integer then return false; end if;
 end loop;
 return true;
end $$;

create function public.biodays_update_feedback(p_id text,p_version integer,p_patch jsonb,p_actor text,p_tokens jsonb,p_mail jsonb) returns jsonb language plpgsql set search_path = public as $$
declare f biodays_feedback; previous text;
begin
 select * into f from biodays_feedback where id=p_id for update;
 if not found then raise exception 'NOT_FOUND'; end if;
 if f.version<>p_version then raise exception 'VERSION_CONFLICT'; end if;
 previous:=f.status;
 update biodays_feedback set status=coalesce(p_patch->>'status',status),internal_note=coalesce(p_patch->>'internal_note',internal_note),public_reply=coalesce(p_patch->>'public_reply',public_reply),version=version+1,updated_at=now(),closed_at=case when coalesce(p_patch->>'status',status) in ('completed','rejected','spam') then coalesce(closed_at,now()) else null end where id=p_id returning * into f;
 insert into biodays_events(feedback_id,seq,from_status,to_status,internal_note,public_reply,actor) values(f.id,f.version,previous,f.status,f.internal_note,f.public_reply,p_actor);
 if f.status='spam' then
  update biodays_outbox set status='cancelled',payload=null,recipient='',updated_at=now() where feedback_id=p_id and status in ('pending','failed','unknown');
 elsif f.subscription_status='confirmed' and f.status in ('accepted','completed','rejected') and previous<>f.status then
  perform biodays_add_tokens(p_tokens); perform biodays_enqueue(p_mail);
 end if;
 return to_jsonb(f);
end $$;

create function public.biodays_subscription(p_hash text,p_purpose text) returns jsonb language plpgsql set search_path = public as $$
declare t biodays_tokens; f biodays_feedback;
begin
 -- Always lock feedback first, matching resend/update/delete lock order.
 select * into t from biodays_tokens where token_hash=p_hash and purpose=p_purpose;
 if not found then raise exception 'TOKEN_INVALID'; end if;
 select * into f from biodays_feedback where id=t.feedback_id for update;
 select * into t from biodays_tokens where token_hash=p_hash for update;
 if not found or f.status='spam' then raise exception 'TOKEN_INVALID'; end if;
 if t.expires_at<now() then raise exception 'TOKEN_EXPIRED'; end if;
 if t.consumed_at is not null then
  if (p_purpose='confirm' and f.subscription_status='confirmed') or (p_purpose='unsubscribe' and f.subscription_status='unsubscribed') then
   return jsonb_build_object('subscription_status',f.subscription_status,'result',case when p_purpose='confirm' then 'already_confirmed' else 'already_unsubscribed' end);
  end if;
  raise exception 'TOKEN_INVALID';
 end if;
 if p_purpose='confirm' and f.subscription_status<>'pending_confirmation' then raise exception 'TOKEN_INVALID'; end if;
 update biodays_tokens set consumed_at=now() where token_hash=p_hash;
 update biodays_feedback set subscription_status=case when p_purpose='confirm' then 'confirmed' else 'unsubscribed' end,confirmed_at=case when p_purpose='confirm' then now() else confirmed_at end,unsubscribed_at=case when p_purpose='unsubscribe' then now() else unsubscribed_at end,updated_at=now(),version=version+1 where id=f.id returning * into f;
 -- seq follows the row version, including subscription events.
 insert into biodays_events(feedback_id,seq,from_status,to_status,actor) values(f.id,f.version,f.status,f.status,'subscriber');
 if p_purpose='unsubscribe' then
  delete from biodays_tokens where feedback_id=f.id and purpose='confirm';
  update biodays_outbox set status='cancelled',payload=null,recipient='',updated_at=now() where feedback_id=f.id and type in ('subscription_confirm','user_status_update') and status in ('pending','failed','unknown');
 end if;
 return jsonb_build_object('subscription_status',f.subscription_status,'result',case when p_purpose='confirm' then 'confirmed' else 'unsubscribed' end);
end $$;

create function public.biodays_resend(p_id text,p_email text,p_tokens jsonb,p_mail jsonb) returns void language plpgsql set search_path = public as $$
declare f biodays_feedback;
begin
 select * into f from biodays_feedback where id=p_id for update;
 if not found or f.email<>p_email or f.subscription_status<>'pending_confirmation' or f.status='spam' then return; end if;
 -- A message already being sent cannot be recalled; don't invalidate its token mid-send.
 if exists(select 1 from biodays_outbox where feedback_id=p_id and type='subscription_confirm' and status='sending') then return; end if;
 delete from biodays_tokens where feedback_id=p_id and purpose='confirm';
 update biodays_outbox set status='cancelled',payload=null,recipient='',updated_at=now() where feedback_id=p_id and type='subscription_confirm' and status in ('pending','failed','unknown');
 perform biodays_add_tokens(p_tokens); perform biodays_enqueue(p_mail);
end $$;

create function public.biodays_delete(p_id text,p_actor text) returns void language plpgsql set search_path = public as $$
begin
 perform 1 from biodays_feedback where id=p_id for update;
 if not found then raise exception 'NOT_FOUND'; end if;
 if exists(select 1 from biodays_outbox where feedback_id=p_id and status='sending') then raise exception 'MAIL_IN_FLIGHT'; end if;
 delete from biodays_feedback where id=p_id;
 insert into biodays_deletions(feedback_id,actor) values(p_id,p_actor);
end $$;

create function public.biodays_claim(p_limit integer default 5) returns setof public.biodays_outbox language plpgsql set search_path = public as $$
begin
 -- An expired lease may have sent successfully; never blindly reclaim it.
 update biodays_outbox set status='unknown',last_error='Lease expired; reconcile provider delivery before retry',lease_until=null,updated_at=now() where status='sending' and lease_until<now();
 update biodays_outbox set status='unknown',last_error='Idempotency window expired; reconcile before retry',updated_at=now() where status='pending' and first_attempt_at<now()-interval '23 hours';
 update biodays_outbox o set status='cancelled',payload=null,recipient='',updated_at=now()
 where status='pending' and (exists(select 1 from biodays_suppressions s where s.email=o.recipient) or (o.type='user_status_update' and not exists(select 1 from biodays_feedback f where f.id=o.feedback_id and f.subscription_status='confirmed' and f.status<>'spam')) or (o.type='subscription_confirm' and not exists(select 1 from biodays_feedback f join biodays_tokens t on t.feedback_id=f.id where f.id=o.feedback_id and f.subscription_status='pending_confirmation' and f.status<>'spam' and t.purpose='confirm' and t.consumed_at is null and t.expires_at>now())));
 return query update biodays_outbox o set status='sending',attempts=attempts+1,lease_until=now()+interval '5 minutes',lease_token=gen_random_uuid(),first_attempt_at=coalesce(first_attempt_at,now()),updated_at=now()
 where id in (select id from biodays_outbox where status='pending' and next_attempt_at<=now() order by next_attempt_at for update skip locked limit least(greatest(p_limit,1),10)) returning o.*;
end $$;

create function public.biodays_finish(p_id text,p_lease uuid,p_status text,p_error text,p_message_id text default null,p_payload jsonb default null) returns void language plpgsql set search_path = public as $$
begin
 if p_status not in ('sent','failed','unknown','pending') then raise exception 'INVALID_STATUS'; end if;
 update biodays_outbox set status=case when p_status='pending' and attempts>=5 then 'failed' else p_status end,last_error=p_error,provider_message_id=coalesce(p_message_id,provider_message_id),provider_accepted_at=case when p_status='sent' then now() else provider_accepted_at end,payload=case when p_status='sent' then null else coalesce(p_payload,payload) end,recipient=case when p_status='sent' then '' else recipient end,lease_until=null,lease_token=null,next_attempt_at=now()+case attempts when 1 then interval '1 minute' when 2 then interval '5 minutes' when 3 then interval '30 minutes' else interval '120 minutes' end,updated_at=now() where id=p_id and status='sending' and lease_token=p_lease;
 if not found then raise exception 'LEASE_LOST'; end if;
 if p_status='sent' then
  update biodays_outbox set delivery_rank=coalesce((select max(case type when 'email.sent' then 1 when 'email.delivered' then 2 when 'email.failed' then 3 when 'email.suppressed' then 4 when 'email.bounced' then 5 when 'email.complained' then 6 else 0 end) from biodays_webhooks where message_id=p_message_id),0) where id=p_id;
  update biodays_outbox set delivery_status=case delivery_rank when 1 then 'email.sent' when 2 then 'email.delivered' when 3 then 'email.failed' when 4 then 'email.suppressed' when 5 then 'email.bounced' when 6 then 'email.complained' else '' end where id=p_id;
 end if;
end $$;

create function public.biodays_retry(p_id text) returns void language plpgsql set search_path = public as $$
declare o biodays_outbox;
begin
 select * into o from biodays_outbox where id=p_id for update;
 if not found then raise exception 'NOT_FOUND'; end if;
 -- Never re-send uncertain or old sends from a UI button. Reconcile in Resend first.
 if o.status<>'failed' or o.first_attempt_at<now()-interval '23 hours' or o.provider_message_id is not null then raise exception 'RECONCILIATION_REQUIRED'; end if;
 update biodays_outbox set status='pending',attempts=0,next_attempt_at=now(),updated_at=now() where id=p_id;
end $$;

create function public.biodays_webhook(p_id text,p_message_id text,p_type text,p_recipients jsonb) returns void language plpgsql set search_path = public as $$
declare rank integer; inserted integer;
begin
 insert into biodays_webhooks(id,message_id,type) values(p_id,p_message_id,p_type) on conflict do nothing;
 get diagnostics inserted=row_count; if inserted=0 then return; end if;
 rank:=case p_type when 'email.sent' then 1 when 'email.delivered' then 2 when 'email.failed' then 3 when 'email.suppressed' then 4 when 'email.bounced' then 5 when 'email.complained' then 6 else 0 end;
 update biodays_outbox set delivery_status=p_type,delivery_rank=rank,updated_at=now() where provider_message_id=p_message_id and delivery_rank<rank;
 if p_type in ('email.bounced','email.complained','email.suppressed') then
  insert into biodays_suppressions(email,reason) select lower(value),p_type from jsonb_array_elements_text(p_recipients) on conflict(email) do update set reason=excluded.reason;
 end if;
end $$;

create function public.biodays_cleanup() returns void language plpgsql set search_path = public as $$
begin
 update biodays_outbox set status='unknown',last_error='Lease expired; reconcile provider delivery before retry',lease_until=null,updated_at=now() where status='sending' and lease_until<now();
 delete from biodays_abuse where expires_at<now();
 delete from biodays_tokens where expires_at<now()-interval '1 day';
 update biodays_outbox set status='cancelled',payload=null,recipient='',updated_at=now() where type='subscription_confirm' and status in ('pending','failed','unknown') and not exists(select 1 from biodays_tokens t where t.feedback_id=biodays_outbox.feedback_id and t.purpose='confirm' and t.expires_at>now());
 delete from biodays_outbox where status in ('sent','cancelled') and updated_at<now()-interval '30 days';
 update biodays_outbox set payload=null,recipient='',status='cancelled',updated_at=now() where created_at<now()-interval '30 days' and status in ('failed','unknown','pending');
 insert into biodays_deletions(feedback_id,actor) select id,'retention' from biodays_feedback f where closed_at<now()-interval '90 days' and not exists(select 1 from biodays_outbox o where o.feedback_id=f.id and o.status='sending');
 delete from biodays_feedback f where closed_at<now()-interval '90 days' and not exists(select 1 from biodays_outbox o where o.feedback_id=f.id and o.status='sending');
 delete from biodays_webhooks where created_at<now()-interval '30 days';
end $$;

-- Existing projects must not inherit broad default grants. This migration
-- changes only biodays objects, never grants access to other application tables.
do $$ declare r record; begin
 for r in select tablename from pg_tables where schemaname='public' and tablename like 'biodays_%' loop
  execute format('alter table public.%I enable row level security',r.tablename);
  execute format('revoke all on public.%I from public,anon,authenticated',r.tablename);
  execute format('grant all on public.%I to service_role',r.tablename);
 end loop;
 for r in select oid::regprocedure as signature from pg_proc where pronamespace='public'::regnamespace and proname like 'biodays_%' loop
  execute format('revoke all on function %s from public,anon,authenticated',r.signature);
  execute format('grant execute on function %s to service_role',r.signature);
 end loop;
 for r in select sequencename from pg_sequences where schemaname='public' and sequencename like 'biodays_%' loop
  execute format('revoke all on sequence public.%I from public,anon,authenticated',r.sequencename);
  execute format('grant all on sequence public.%I to service_role',r.sequencename);
 end loop;
end $$;
