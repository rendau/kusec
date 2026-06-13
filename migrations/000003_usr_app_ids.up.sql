alter table usr
    add column if not exists app_ids jsonb not null default '[]'::jsonb;

drop table if exists usr_app cascade;
