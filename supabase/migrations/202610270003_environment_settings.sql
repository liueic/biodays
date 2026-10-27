-- Deploy credentials through environment variables before upgrading the app.
-- Preserve effective business settings and feedback while removing legacy values.
do $$
declare
  r public.biodays_settings;
  cleaned jsonb;
begin
  select * into r from public.biodays_settings where id for update;
  if not found then return; end if;
  cleaned := r.document
    #- '{resend,api_key}' #- '{resend,from}' #- '{resend,notify_to}'
    #- '{turnstile,site_key}' #- '{turnstile,secret_key}' #- '{turnstile,hostnames}'
    #- '{ai_spam,api_key}' #- '{ai_spam,api_token}' #- '{ai_spam,account_id}' #- '{ai_spam,style}';
  -- Removing legacy Cloudflare markers must not accidentally activate Jev.
  if (r.document->'ai_spam') ?| array['api_token', 'account_id', 'style'] then
    cleaned := jsonb_set(cleaned, '{ai_spam,enabled}', 'false'::jsonb);
  end if;
  if cleaned is distinct from r.document then
    perform public.biodays_save_settings(
      r.revision, cleaned, 'environment-settings-migration',
      '{"deployment_settings":"Removed legacy deployment fields; environment variables are authoritative"}'
    );
  end if;
end $$;
