-- Vistas semanticas canonicas para Sabio.
-- Aplicar sobre una copia o migracion controlada de panalbit.db si se quieren
-- consultar como vw_*. Sabio tambien usa este archivo como guia de SQL canonico.

CREATE VIEW IF NOT EXISTS vw_law_firms AS
SELECT
  "id" AS law_firm_id,
  "name" AS estudio_juridico,
  "has_metadata" AS tiene_metadata
FROM "law_firms";

CREATE VIEW IF NOT EXISTS vw_clients_overview AS
SELECT
  c."id" AS client_id,
  c."name" AS cliente,
  c."code" AS codigo_cliente,
  c."active" AS activo,
  c."agreement_start_date" AS fecha_inicio_acuerdo,
  (SELECT COUNT(*) FROM "projects" p WHERE p."client_id" = c."id") AS proyectos_count,
  (SELECT COUNT(*) FROM "charges" ch WHERE ch."client_id" = c."id") AS cargos_count,
  (SELECT COUNT(*) FROM "billing_documents" bd WHERE bd."client_id" = c."id") AS documentos_facturacion_count,
  (SELECT COUNT(*) FROM "payments" pay WHERE pay."client_id" = c."id") AS pagos_count,
  (SELECT COUNT(*) FROM "expenses" e WHERE e."client_id" = c."id") AS gastos_count,
  COALESCE((SELECT SUM(CAST(pay."amount" AS REAL)) FROM "payments" pay WHERE pay."client_id" = c."id"), 0) AS pagos_amount
FROM "clients" c
GROUP BY c."id";

CREATE VIEW IF NOT EXISTS vw_projects_overview AS
SELECT
  p."id" AS project_id,
  p."code" AS codigo_proyecto,
  p."name" AS proyecto,
  p."active" AS activo,
  p."collectable" AS cobrable,
  p."currency_code" AS moneda_codigo,
  c."id" AS client_id,
  c."name" AS cliente,
  a."id" AS agreement_id,
  a."name" AS acuerdo,
  pa."name" AS area_proyecto,
  pt."name" AS tipo_proyecto,
  COUNT(DISTINCT te."id") AS registros_horas_count,
  COALESCE(SUM(CAST(te."duration" AS REAL)), 0) AS horas_total_hours,
  COALESCE(SUM(CAST(te."billable_duration" AS REAL)), 0) AS horas_facturables_hours
FROM "projects" p
LEFT JOIN "clients" c ON c."id" = p."client_id"
LEFT JOIN "agreements" a ON a."id" = p."agreement_id"
LEFT JOIN "project_areas" pa ON pa."id" = p."project_area_id"
LEFT JOIN "project_types" pt ON pt."id" = p."project_type_id"
LEFT JOIN "time_entries" te ON te."project_code" = p."code"
GROUP BY p."id";

CREATE VIEW IF NOT EXISTS vw_collection_overview AS
SELECT
  c."id" AS client_id,
  c."name" AS cliente,
  c."code" AS codigo_cliente,
  COALESCE(SUM(CAST(m."amount" AS REAL)), 0) AS saldo_amount,
  CAST((julianday('now') - julianday(MIN(m."date"))) AS INTEGER) AS mora_dias,
  COUNT(DISTINCT ch."id") AS cargos_impagos_count,
  COUNT(DISTINCT bd."id") AS documentos_facturacion_count
FROM "clients" c
JOIN "charges" ch ON ch."client_id" = c."id"
LEFT JOIN "milestones" m ON m."charge_id" = ch."id"
LEFT JOIN "billing_documents" bd ON bd."charge_id" = ch."id"
WHERE ch."state" IN (
  'FACTURADO',
  'EMITIDO',
  'PAGO PARCIAL',
  'ENVIADO AL CLIENTE',
  'EN REVISION'
)
AND m."amount" IS NOT NULL
AND m."amount" != ''
GROUP BY c."id";

CREATE VIEW IF NOT EXISTS vw_time_entries_overview AS
SELECT
  te."id" AS time_entry_id,
  te."date" AS fecha,
  te."duration" AS duracion_hours,
  te."billable_duration" AS duracion_facturable_hours,
  te."notes" AS notas,
  p."id" AS project_id,
  p."code" AS codigo_proyecto,
  p."name" AS proyecto,
  c."id" AS client_id,
  c."name" AS cliente,
  u."id" AS user_id,
  u."name" AS usuario_interno,
  u."email" AS email_usuario_interno
FROM "time_entries" te
LEFT JOIN "projects" p ON p."code" = te."project_code"
LEFT JOIN "clients" c ON c."id" = p."client_id"
LEFT JOIN "users" u ON u."id" = te."user_id";
