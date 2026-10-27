\set ON_ERROR_STOP on
begin;
create function pg_temp.assert(ok boolean, message text) returns void language plpgsql as $$ begin if ok is distinct from true then raise exception 'Assertion failed: %',message; end if; end $$;
create function pg_temp.feedback(id text, req text, email text default 'reader@example.com') returns jsonb language sql as $$ select jsonb_build_object('id',id,'kind','issue','title','标题','content','这里是一个完整的反馈正文，请进行处理','content_hash','hash','email_domain','example.com','source_url','','story_id','Q1-01','nickname','','email',email,'status','pending','subscription_status','pending_confirmation','confirmation_email','queued','consent_version','test','consented_at',now(),'spam_score',0,'spam_verdict','allow','spam_signals','[]'::jsonb,'spam_source','heuristic','spam_model','','spam_confidence',0,'verification_method','none'); $$;
create function pg_temp.token(id text, h text, purpose text default 'confirm') returns jsonb language sql as $$ select jsonb_build_array(jsonb_build_object('token_hash',h,'feedback_id',id,'purpose',purpose,'expires_at',now()+interval '1 day')); $$;
create function pg_temp.mail(id text, fb text, typ text, event_key text) returns jsonb language sql as $$ select jsonb_build_array(jsonb_build_object('id',id,'feedback_id',fb,'type',typ,'event_key',event_key,'recipient','reader@example.com','payload',jsonb_build_object('subject','test','text','private payload','html','private payload'))); $$;

-- No anonymous or normal authenticated user can read PII or invoke commands.
select pg_temp.assert(not has_table_privilege('anon','biodays_feedback','select'),'anon feedback select revoked');
select pg_temp.assert(not has_table_privilege('authenticated','biodays_admins','insert'),'self-promotion denied');
select pg_temp.assert(not has_function_privilege('anon','biodays_subscription(text,text)','execute'),'anon RPC revoked');
select pg_temp.assert(not exists(select 1 from pg_tables where schemaname='public' and tablename like 'biodays_%' and not rowsecurity),'RLS on all business tables');
select pg_temp.assert(not has_function_privilege('anon','biodays_submit(jsonb,text,text,jsonb,jsonb)','execute'),'updated Jev RPC retains ACL');

select biodays_submit(pg_temp.feedback('jev','jev') || '{"spam_probability":0.8,"spam_model":"typesafe-ai/jev-1.14.0","spam_source":"ai"}', 'jev','jev','[]','[]');
select pg_temp.assert((select spam_probability=0.8 and spam_confidence=0 from biodays_feedback where id='jev'),'probability saved independently from legacy confidence');
select biodays_submit(pg_temp.feedback('ignored','jev') || '{"spam_probability":0.1}', 'jev','jev','[]','[]');
select pg_temp.assert((select spam_probability=0.8 from biodays_feedback where id='jev'),'duplicate submission keeps original probability');
do $$begin
 begin perform biodays_submit(pg_temp.feedback('bad-jev','bad-jev') || '{"spam_probability":1.1}', 'bad-jev','bad-jev','[]','[]');raise exception 'Expected invalid probability';exception when check_violation then null;end;
end$$;
delete from biodays_feedback where id='jev';

select biodays_submit(pg_temp.feedback('f1','r1'),'r1','h1',pg_temp.token('f1','confirm1'),pg_temp.mail('m1','f1','subscription_confirm','confirm:f1'));
select biodays_submit(pg_temp.feedback('discarded','r1'),'r1','h1','[]','[]');
select pg_temp.assert((select count(*)=1 from biodays_feedback),'same request returns same row');
select pg_temp.assert((select spam_probability is null from biodays_feedback where id='f1'),'historical confidence is not converted to probability');
select pg_temp.assert((select count(*)=1 from biodays_outbox),'same request does not duplicate mail');
do $$begin
 begin perform biodays_submit(pg_temp.feedback('f2','r1'),'r1','different','[]','[]');raise exception 'Expected idempotency conflict';exception when others then if sqlerrm<>'IDEMPOTENCY_CONFLICT' then raise;end if;end;
 begin perform biodays_submit(pg_temp.feedback('rollback','rollback'),'rollback','h2',pg_temp.token('missing','bad'),'[]');raise exception 'Expected FK failure';exception when foreign_key_violation then null;end;
end$$;
select pg_temp.assert(not exists(select 1 from biodays_feedback where id='rollback'),'failed token rolls back feedback');
select pg_temp.assert(biodays_subscription('confirm1','confirm')->>'result'='confirmed','double opt-in confirmed');
select pg_temp.assert(biodays_subscription('confirm1','confirm')->>'result'='already_confirmed','confirmation replay safe');
select biodays_update_feedback('f1',2,'{"status":"accepted"}','admin',pg_temp.token('f1','unsub1','unsubscribe'),pg_temp.mail('m2','f1','user_status_update','status:f1:3'));
do $$begin
 begin perform biodays_update_feedback('f1',2,'{"status":"completed"}','admin','[]','[]');raise exception 'Expected version conflict';exception when others then if sqlerrm<>'VERSION_CONFLICT' then raise;end if;end;
end$$;
select pg_temp.assert((select count(*)=3 from biodays_events where feedback_id='f1'),'feedback audit seq includes subscriber event');
select pg_temp.assert(biodays_subscription('unsub1','unsubscribe')->>'result'='unsubscribed','unsubscribe works');
select pg_temp.assert((select status='cancelled' and payload is null from biodays_outbox where id='m2'),'unsubscribe cancels queued status mail and sensitive payload');
select pg_temp.assert(biodays_subscription('unsub1','unsubscribe')->>'result'='already_unsubscribed','unsubscribe replay safe');

select biodays_submit(pg_temp.feedback('f3','r3'),'r3','h3',pg_temp.token('f3','oldconfirm'),'[]');
select biodays_resend('f3','reader@example.com',pg_temp.token('f3','newconfirm'),pg_temp.mail('m3','f3','subscription_confirm','confirm:f3:new'));
select pg_temp.assert(not exists(select 1 from biodays_tokens where token_hash='oldconfirm'),'resend invalidates previous token');
select pg_temp.assert(exists(select 1 from biodays_tokens where token_hash='newconfirm'),'resend creates new token');

insert into biodays_settings(id,document) values(true,'{}');
select biodays_save_settings(0,'{"resend":{"enabled":false}}','admin','{"resend":"changed"}');
do $$begin
 begin perform biodays_save_settings(0,'{}','stale-admin','{}');raise exception 'Expected settings conflict';exception when others then if sqlerrm<>'VERSION_CONFLICT' then raise;end if;end;
end$$;
select pg_temp.assert((select revision=1 from biodays_settings),'settings CAS revision');
select pg_temp.assert((select count(*)=1 from biodays_settings_audit),'failed save has no audit');

select pg_temp.assert(biodays_rate('[{"key":"ip1","start":"2026-01-01","expires":"2027-01-01","limit":1}]'),'first rate attempt allowed');
select pg_temp.assert(not biodays_rate('[{"key":"ip1","start":"2026-01-01","expires":"2027-01-01","limit":1}]'),'second rate attempt rejected');

-- Lease uniqueness, immutable event keys, successful payload clearing, retry guard.
select biodays_enqueue(pg_temp.mail('m4',null,'resend_test','test:4'));
select biodays_enqueue(pg_temp.mail('duplicate',null,'resend_test','test:4'));
select pg_temp.assert((select count(*)=1 from biodays_outbox where event_key='test:4'),'unique durable mail event');
create temporary table claimed as select * from biodays_claim(10);
select pg_temp.assert((select count(*)=2 from claimed),'only eligible confirmation and test claimed');
select pg_temp.assert((select count(*)=0 from biodays_claim(10)),'cannot claim twice');
select biodays_finish('m4',(select lease_token from claimed where id='m4'),'sent','','provider-1');
select pg_temp.assert((select payload is null and recipient='' and status='sent' from biodays_outbox where id='m4'),'accepted mail clears PII');
select biodays_webhook('event-1','provider-1','email.complained','["reader@example.com"]');
select biodays_webhook('event-2','provider-1','email.delivered','["reader@example.com"]');
select biodays_webhook('event-1','provider-1','email.complained','["reader@example.com"]');
select pg_temp.assert((select delivery_status='email.complained' from biodays_outbox where id='m4'),'out-of-order webhook cannot overwrite terminal complaint');
select pg_temp.assert((select count(*)=2 from biodays_webhooks),'webhook replay deduplicated');
select pg_temp.assert(exists(select 1 from biodays_suppressions where email='reader@example.com'),'complaint suppresses future mail');
update biodays_outbox set lease_until=now()-interval '1 minute' where id='m3';
select * from biodays_claim(10);
select pg_temp.assert((select status='unknown' from biodays_outbox where id='m3'),'expired lease never silently re-sent');
do $$begin
 begin perform biodays_retry('m3');raise exception 'Expected reconcile requirement';exception when others then if sqlerrm<>'RECONCILIATION_REQUIRED' then raise;end if;end;
end$$;
-- Out-of-window sends and stale worker completions must never be re-sent/accepted.
select biodays_enqueue(pg_temp.mail('old-task',null,'resend_test','test:old'));
update biodays_outbox set first_attempt_at=now()-interval '25 hours' where id='old-task';
select * from biodays_claim(10);
select pg_temp.assert((select status='unknown' and attempts=0 from biodays_outbox where id='old-task'),'expired idempotency window not retried');
do $$begin
 begin perform biodays_finish('m4',gen_random_uuid(),'sent','','other-provider');raise exception 'Expected lease guard';exception when others then if sqlerrm<>'LEASE_LOST' then raise;end if;end;
end$$;
-- Retention cleans closed feedback and leaves only minimal deletion audit.
update biodays_feedback set closed_at=now()-interval '91 days' where id='f1';
select biodays_cleanup();
select pg_temp.assert(not exists(select 1 from biodays_feedback where id='f1'),'old closed feedback removed');
select pg_temp.assert(exists(select 1 from biodays_deletions where feedback_id='f1'),'deletion audit retained');
rollback;
\echo 'Database regression checks passed'
