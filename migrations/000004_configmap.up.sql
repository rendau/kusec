create table configmap (
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

-- Имя k8s-configmap: {app.slug_name}-{configmap.slug_name} в namespace
-- приложения, поэтому пара (app_id, slug_name) уникальна.
create unique index uq_configmap_app_id_slug_name on configmap (app_id, slug_name);

create table config_item (
    id           text        not null default gen_random_uuid()::text,
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now(),
    configmap_id text        not null,
    active       boolean     not null default true,
    key          text        not null default '',
    value        text        not null default '',
    value_format text        not null default 'text',
    encoding     text        not null default 'plain',
    file_name    text        not null default '',
    content_type text        not null default '',
    description  text        not null default '',
    primary key (id),
    foreign key (configmap_id) references configmap (id) on delete cascade
);

create unique index uq_config_item_configmap_id_key on config_item (configmap_id, key);
