-- Тип k8s-секрета (пусто = Opaque). Нужен для секретов вроде
-- kubernetes.io/basic-auth, которые требует traefik.
alter table secret
    add column kube_type text not null default '';
