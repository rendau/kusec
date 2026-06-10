create table usrs (
    id       bigserial not null,
    active   boolean   not null default true,
    is_admin boolean   not null default false,
    name     text      not null default '',
    username text      not null default '',
    password text      not null default '',
    primary key (id)
);

create unique index uq_usrs_username on usrs (username);
