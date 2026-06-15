-- exact_slug: при true имя итогового k8s-объекта = slug_name без префикса
-- KUBE_SECRET_NAME_PREFIX и без app-slug. Менять флаг могут только админы.
alter table secret
    add column exact_slug boolean not null default false;

alter table configmap
    add column exact_slug boolean not null default false;
