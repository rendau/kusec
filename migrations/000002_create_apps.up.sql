create table apps (
    id          text        not null default gen_random_uuid()::text,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    active      boolean     not null default true,
    name        text        not null default '',
    description text        not null default '',
    primary key (id)
);

create unique index uq_apps_name on apps (name);
