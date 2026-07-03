-- API-ключи: долгоживущие учётные данные для машинных клиентов (MCP-сервер,
-- CI и т.п.). Хранится только sha256-хэш ключа; сам ключ показывается один раз
-- при создании. Ключ наследует права владельца (usr).
create table api_key (
    id           text        not null default gen_random_uuid()::text,
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now(),
    usr_id       bigint      not null,
    active       boolean     not null default true,
    -- mcp_only: ключ принимается только встроенным MCP-эндпоинтом,
    -- основной API его отвергает (агент не может обойти маскирование).
    mcp_only     boolean     not null default false,
    name         text        not null default '',
    key_hash     text        not null,
    key_prefix   text        not null default '',
    last_used_at timestamptz,
    primary key (id),
    foreign key (usr_id) references usr (id) on delete cascade
);

create unique index uq_api_key_key_hash on api_key (key_hash);
create index ix_api_key_usr_id on api_key (usr_id);
