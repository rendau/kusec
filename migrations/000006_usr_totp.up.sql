alter table usr
    add column totp_secret  text    not null default '',
    add column totp_enabled boolean not null default false;
