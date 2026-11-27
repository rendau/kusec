-- Колонки возвращаются пустыми: удалённые ключи 2FA не восстановить.
alter table usr
    add column if not exists totp_secret  text    not null default '',
    add column if not exists totp_enabled boolean not null default false;
