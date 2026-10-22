-- Подключение OAuth-клиента (например, Claude) к встроенному MCP-серверу,
-- разрешённое пользователем на странице согласия админки. Токены подключения
-- работают как mcp_only-ключ этого пользователя. Подключение без connected_at
-- ждёт обмена кода на токены.
create table oauth_grant (
    id             text        not null default gen_random_uuid()::text,
    usr_id         bigint      not null,
    client_id      text        not null,
    client_name    text        not null default '',
    redirect_uri   text        not null default '',
    code_challenge text        not null default '',
    created_at     timestamptz not null default now(),
    connected_at   timestamptz,
    last_used_at   timestamptz,
    primary key (id),
    foreign key (usr_id) references usr (id) on delete cascade
);

create index ix_oauth_grant_usr_id on oauth_grant (usr_id);

-- Коды авторизации, access- и refresh-токены подключений. Хранится только
-- sha256 токена.
create table oauth_token (
    token_hash text        not null,
    grant_id   text        not null,
    kind       text        not null,
    expires_at timestamptz not null,
    created_at timestamptz not null default now(),
    primary key (token_hash),
    foreign key (grant_id) references oauth_grant (id) on delete cascade
);

create index ix_oauth_token_grant_id on oauth_token (grant_id);
create index ix_oauth_token_expires_at on oauth_token (expires_at);
