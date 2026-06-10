create table usr (
    id       bigserial not null,
    active   boolean   not null default true,
    is_admin boolean   not null default false,
    name     text      not null default '',
    username text      not null default '',
    password text      not null default '',
    primary key (id)
);

create unique index uq_usr_username on usr (username);

create table app (
    id          text        not null default gen_random_uuid()::text,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    active      boolean     not null default true,
    namespace   text        not null default '',
    name        text        not null default '',
    description text        not null default '',
    primary key (id)
);

create unique index uq_app_name on app (name);

create table secret (
    id          text        not null default gen_random_uuid()::text,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    app_id      text        not null,
    active      boolean     not null default true,
    slug_name   text        not null default '',
    description text        not null default '',
    primary key (id),
    foreign key (app_id) references app (id) on delete cascade
);

create unique index uq_secret_app_id_slug_name on secret (app_id, slug_name);

create table item (
    id          text        not null default gen_random_uuid()::text,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    secret_id   text        not null,
    active      boolean     not null default true,
    key         text        not null default '',
    value       text        not null default '',
    description text        not null default '',
    primary key (id),
    foreign key (secret_id) references secret (id) on delete cascade
);

create unique index uq_item_secret_id_key on item (secret_id, key);
