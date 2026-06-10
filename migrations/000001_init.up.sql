create table usrs (
    id       bigserial not null,
    active   boolean   not null default true,
    is_admin boolean   not null default false,
    name     text      not null default '',
    username text      not null default '',
    password text      not null default '',
    primary key (id)
);

create unique index uq_usrs_username on usrs (username);

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

create table items (
    id          text        not null default gen_random_uuid()::text,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    app_id      text        not null,
    active      boolean     not null default true,
    key         text        not null default '',
    value       text        not null default '',
    description text        not null default '',
    primary key (id),
    foreign key (app_id) references apps (id) on delete cascade
);

create unique index uq_items_app_id_key on items (app_id, key);
