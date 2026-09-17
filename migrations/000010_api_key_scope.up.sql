-- Скоуп API-ключа вместо булевого mcp_only:
--   full      — полный доступ (наследует права владельца);
--   read_only — только методы чтения из белого списка, значения маскируются;
--   mcp_only  — ключ принимается только встроенным MCP-эндпоинтом.
alter table api_key add column scope text not null default 'full';

update api_key set scope = 'mcp_only' where mcp_only;

alter table api_key drop column mcp_only;
