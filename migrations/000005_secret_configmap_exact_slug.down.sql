alter table configmap
    drop column if exists exact_slug cascade;

alter table secret
    drop column if exists exact_slug cascade;
