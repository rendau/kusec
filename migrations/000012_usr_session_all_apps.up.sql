-- session_version — номер поколения сессий: зашит в access- и refresh-токены
-- (claim ver), увеличивается при смене пароля, выключении, выходе, включении
-- и сбросе 2FA — все выданные раньше токены перестают приниматься.
-- all_apps — не-админ видит все приложения; иначе только app_ids, пустой
-- список = не видит ничего.
alter table usr
    add column if not exists session_version bigint  not null default 0,
    add column if not exists all_apps        boolean not null default false;

-- до этой миграции пустой app_ids у не-админа означал «доступ ко всем»:
-- сохраняем поведение существующих пользователей явным флагом
update usr set all_apps = true where not is_admin and app_ids = '[]'::jsonb;

-- токены, выданные до этой миграции, считаются поколением 0. У выключенных
-- пользователей поколение сдвигается сразу: иначе их старые refresh-токены
-- (живут долго) снова заработали бы после повторного включения учётки —
-- выключение до этой миграции номер не увеличивало
update usr set session_version = 1 where not active;
