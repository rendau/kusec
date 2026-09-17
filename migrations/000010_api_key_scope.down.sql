alter table api_key add column mcp_only boolean not null default false;

update api_key set mcp_only = true where scope = 'mcp_only';

alter table api_key drop column if exists scope cascade;
