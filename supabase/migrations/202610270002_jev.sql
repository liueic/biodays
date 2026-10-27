-- Additive migration: historical classifier confidence is not P(spam).
alter table public.biodays_feedback
 add column spam_probability double precision
 check (spam_probability >= 0 and spam_probability <= 1);

-- Keep the same signature, ACL, request lock, events and transactional outbox.
create or replace function public.biodays_submit(p_feedback jsonb,p_request_id text,p_hash text,p_tokens jsonb,p_mail jsonb) returns jsonb language plpgsql set search_path = public as $$
declare f biodays_feedback;
begin
 perform pg_advisory_xact_lock(hashtextextended(p_request_id,0));
 select * into f from biodays_feedback where request_id=p_request_id;
 if found then
  if f.request_body_hash<>p_hash then raise exception 'IDEMPOTENCY_CONFLICT'; end if;
  return to_jsonb(f)||jsonb_build_object('_created',false);
 end if;
 insert into biodays_feedback(id,request_id,request_body_hash,kind,title,content,content_hash,email_domain,source_url,story_id,nickname,email,status,subscription_status,confirmation_email,consent_version,consented_at,spam_score,spam_verdict,spam_signals,spam_source,spam_model,spam_confidence,spam_probability,verification_method,verified_at)
 values(p_feedback->>'id',p_request_id,p_hash,p_feedback->>'kind',p_feedback->>'title',p_feedback->>'content',p_feedback->>'content_hash',p_feedback->>'email_domain',p_feedback->>'source_url',p_feedback->>'story_id',p_feedback->>'nickname',p_feedback->>'email',p_feedback->>'status',p_feedback->>'subscription_status',p_feedback->>'confirmation_email',p_feedback->>'consent_version',(p_feedback->>'consented_at')::timestamptz,(p_feedback->>'spam_score')::integer,p_feedback->>'spam_verdict',p_feedback->'spam_signals',p_feedback->>'spam_source',p_feedback->>'spam_model',(p_feedback->>'spam_confidence')::real,(p_feedback->>'spam_probability')::double precision,p_feedback->>'verification_method',(p_feedback->>'verified_at')::timestamptz) returning * into f;
 if f.status='spam' then update biodays_feedback set closed_at=now() where id=f.id; end if;
 insert into biodays_events(feedback_id,seq,from_status,to_status,actor) values(f.id,1,'',f.status,'visitor');
 perform biodays_add_tokens(p_tokens); perform biodays_enqueue(p_mail);
 return to_jsonb(f)||jsonb_build_object('_created',true);
end $$;
