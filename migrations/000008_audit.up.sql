-- Аудит изменений: по записи на каждую мутацию (create/update/delete,
-- activate/deactivate, import, sync). Без внешних ключей — записи аудита
-- переживают свои сущности; чистка только фоновым ретеншном.
create table audit (
    id               bigserial   not null,
    created_at       timestamptz not null default now(),
    -- actor: пользователь и/или api-ключ; имя денормализовано (пользователь
    -- или ключ могут быть удалены)
    actor_usr_id     bigint,
    actor_api_key_id text,
    actor_name       text        not null default '',
    source           text        not null default '', -- ui|api|mcp|system
    request_id       text        not null default '',
    -- объект
    entity_type      text        not null default '', -- app|secret|item|configmap|config_item|api_key|usr|sync_run
    entity_id        text        not null default '',
    app_id           text,
    namespace        text        not null default '',
    app_slug         text        not null default '',
    kube_kind        text        not null default '', -- Secret|ConfigMap|''
    kube_name        text        not null default '', -- итоговое имя k8s-объекта
    key              text        not null default '', -- для item/config_item
    action           text        not null default '', -- create|update|delete|activate|deactivate|sync|import
    -- список изменённых полей [{field, old, new} | {field, old_hash, new_hash, old_size, new_size}];
    -- значения item (secret) — только отпечаток и размер
    changes          jsonb       not null default '[]',
    -- группировка массовых операций (каскадное удаление, импорт, sync)
    batch_id         text,
    primary key (id)
);

create index ix_audit_app_id_created_at on audit (app_id, created_at desc);
create index ix_audit_created_at on audit (created_at desc);
create index ix_audit_ns_kube_name_created_at on audit (namespace, kube_name, created_at desc);
