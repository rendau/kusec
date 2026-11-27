-- 2FA убрана: ключи и признак включения больше не нужны.
alter table usr
    drop column if exists totp_enabled cascade,
    drop column if exists totp_secret cascade;
