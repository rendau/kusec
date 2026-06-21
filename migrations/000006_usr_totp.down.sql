alter table usr
    drop column if exists totp_enabled,
    drop column if exists totp_secret;
