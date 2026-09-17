-- Журнал запусков синхронизации kusec → Kubernetes.
create table sync_run (
    id               text        not null default gen_random_uuid()::text,
    started_at       timestamptz not null default now(),
    finished_at      timestamptz,
    status           text        not null default 'running', -- running|ok|partial|error
    error            text        not null default '',
    actor_usr_id     bigint,
    actor_api_key_id text,
    actor_name       text        not null default '',
    source           text        not null default '', -- ui|api|mcp|system
    request_id       text        not null default '',
    -- null — sync всех доступных app
    app_id           text,
    duration_ms      bigint      not null default 0,
    primary key (id)
);

create index ix_sync_run_started_at on sync_run (started_at desc);
create index ix_sync_run_app_id_started_at on sync_run (app_id, started_at desc);

-- Затронутые k8s-объекты одного запуска sync.
create table sync_run_object (
    id           bigserial not null,
    run_id       text      not null,
    namespace    text      not null default '',
    kube_kind    text      not null default '', -- Secret|ConfigMap
    kube_name    text      not null default '',
    op           text      not null default '', -- created|updated|deleted|unchanged|error
    error        text      not null default '',
    -- HMAC-отпечаток содержимого объекта (без значений)
    content_hash text      not null default '',
    -- имена ключей с изменившимся значением (без значений)
    changed_keys text[]    not null default '{}',
    primary key (id),
    foreign key (run_id) references sync_run (id) on delete cascade
);

create index ix_sync_run_object_run_id on sync_run_object (run_id);

-- Последний применённый в кластер снимок: отличает «изменено в kusec,
-- но ещё не применено». Обновляется вне обычного Update (updated_at не трогает).
alter table secret add column last_synced_at timestamptz;
alter table secret add column last_synced_hash text not null default '';
alter table configmap add column last_synced_at timestamptz;
alter table configmap add column last_synced_hash text not null default '';
