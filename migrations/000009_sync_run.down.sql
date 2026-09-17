alter table configmap drop column if exists last_synced_hash cascade;
alter table configmap drop column if exists last_synced_at cascade;
alter table secret drop column if exists last_synced_hash cascade;
alter table secret drop column if exists last_synced_at cascade;

drop table if exists sync_run_object cascade;
drop table if exists sync_run cascade;
