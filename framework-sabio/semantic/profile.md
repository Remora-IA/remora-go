# Perfil de panalbit.db

| Tabla | Filas | Campos | Rol aproximado |
|---|---:|---|---|
| `advances` | 1 | amount, client_code, client_id, date, id, project_code, project_id, residue | Anticipos |
| `agreements` | 614 | client_id, code, id, name | Contratos/acuerdos |
| `areas` | 9 | created_at, id, name, touched_at, updated_at | Áreas generales |
| `bank_accounts` | 3 | address, bank_id, currency_id, fax, id, name, number, phone_1, phone_2 | Cuentas bancarias |
| `banks` | 2 | id, name | Bancos |
| `billing_document_statuses` | 5 | code, id, name | Estados de documentos |
| `billing_document_types` | 6 | code, id, name | Tipos de documentos |
| `billing_documents` | 1579 | charge_id, client_code, client_id, created_at, date, id, number, series_number, updated_at | Facturas/documentos |
| `charges` | 1518 | agreement_id, client_code, client_id, created_at, date_from, date_to, description, id, state, updated_at | Cargos/cobros |
| `client_groups` | 7 | client_code, client_id, country_id, id, name | Grupos de clientes |
| `clients` | 269 | active, agreement_start_date, code, created_at, group_id, id, name, updated_at | Clientes/deudores |
| `codes` | 127 | code, description, group, id | Códigos/clasificadores |
| `countries` | 241 | alpha_2_code, alpha_3_code, id, name_es, numeric_code | Países |
| `currencies` | 3 | billing_document_symbol, code, decimal_char, exchange_rate, id, is_base_currency, name, plural, precision, symbol, thousand_char | Monedas |
| `expenses` | 8 | client_id, created_at, description, id, observations, project_id, reviewed, updated_at | Gastos |
| `expenses_categories` | 21 | id, name | Categorías de gastos |
| `languages` | 2 | code, id, name_es | Idiomas |
| `law_firms` | 2 | has_metadata, id, name | Estudios jurídicos |
| `milestones` | 308 | agreement_id, amount, charge_id, date, description, id | Hitos de cobro |
| `national_id_document_types` | 6 | dte_code, id, name | Tipos de ID nacional |
| `payment_concepts` | 18 | id, name | Conceptos de pago |
| `payment_conditions` | 16 | id, name | Condiciones de pago |
| `payment_types` | 10 | code, group, name | Tipos de pago |
| `payments` | 1451 | amount, client_id, created_at, currency_id, date, id, residue, updated_at | Pagos |
| `project_areas` | 11 | breakdown, id, name | Áreas de proyecto |
| `project_types` | 6 | id, name | Tipos de proyecto |
| `projects` | 521 | active, agreement_id, client_code, client_id, client_name, code, collectable, created_at, currency_code, has_custom_agreement, id, inactive_at, language_code, language_name, name, project_area_id, project_sub_area_id, project_type_id, responsible_user_ids, updated_at | Proyectos/casos/asuntos |
| `providers` | 1 | id, identification, name | Proveedores |
| `rates` | 59 | flat_amount, id, is_default, name, saved | Tarifas |
| `related_document_types` | 3 | id, name | Tipos de documentos relacionados |
| `time_entries` | 30050 | billable_duration, created_at, date, duration, id, notes, project_code, updated_at, user_id | Horas/trabajo |
| `user_areas` | 12 | id, name, related_id | Áreas de usuario |
| `user_categories` | 10 | category_code, english_name, id, name, order | Categorías de usuario |
| `users` | 70 | active, code, cost_centre, email, firstname, id, lastname1, lastname2, name, permissions, phone, roles, settings, user_area_id, user_category_id, username, visible | Usuarios internos |

## Relaciones curadas

- `advances.client_id` → `clients.id` (high; column name + entity table exists)
- `advances.project_id` → `projects.id` (high; column name + entity table exists)
- `agreements.client_id` → `clients.id` (high; known API semantics)
- `bank_accounts.bank_id` → `banks.id` (high; column name + entity table exists)
- `bank_accounts.currency_id` → `currencies.id` (high; column name + entity table exists)
- `billing_documents.charge_id` → `charges.id` (high; known API semantics)
- `billing_documents.client_id` → `clients.id` (high; known API semantics)
- `clients.group_id` → `client_groups.id` (medium; column name; client_groups also has client_id)
- `client_groups.client_id` → `clients.id` (medium; column name)
- `client_groups.country_id` → `countries.id` (high; column name + entity table exists)
- `charges.agreement_id` → `agreements.id` (high; known API semantics)
- `charges.client_id` → `clients.id` (high; known API semantics)
- `expenses.client_id` → `clients.id` (high; known API semantics)
- `expenses.project_id` → `projects.id` (high; known API semantics)
- `milestones.agreement_id` → `agreements.id` (high; known API semantics)
- `milestones.charge_id` → `charges.id` (high; known API semantics)
- `payments.client_id` → `clients.id` (high; known API semantics)
- `payments.currency_id` → `currencies.id` (high; known API semantics)
- `projects.agreement_id` → `agreements.id` (high; known API semantics)
- `projects.client_id` → `clients.id` (high; known API semantics)
- `projects.currency_code` → `currencies.code` (high; known API semantics)
- `projects.language_code` → `languages.code` (medium; column name + language code table)
- `projects.project_area_id` → `project_areas.id` (high; known API semantics)
- `projects.project_type_id` → `project_types.id` (high; known API semantics)
- `time_entries.project_code` → `projects.code` (high; known API semantics)
- `time_entries.user_id` → `users.id` (high; known API semantics)
- `user_areas.related_id` → `areas.id` (low; ambiguous related_id)
- `users.user_area_id` → `user_areas.id` (high; known API semantics)
- `users.user_category_id` → `user_categories.id` (high; known API semantics)

## Tablas sin relación visible o solo lookup no referenciado

- `billing_document_statuses`
- `billing_document_types`
- `codes`
- `expenses_categories`
- `law_firms`
- `national_id_document_types`
- `payment_concepts`
- `payment_conditions`
- `payment_types`
- `providers`
- `rates`
- `related_document_types`
