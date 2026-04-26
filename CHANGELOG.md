# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.2] - 2026-04-25

> **Release**: 5 nuevos frameworks + expansiones de Paladin y Echo

### Nuevo: Framework Charlie

- **Framework de versionado y changelog** para el proyecto Remora
- Sistema de clasificación de cambios (feat, fix, docs, test, chore, etc.)
- Reglas SemVer integradas (major, minor, patch bumps)
- Changelog automático en formato Keep a Changelog
- CLI para verificar estado del repo y proponer commits
- Archivos: INITIAL_PROMPT.md, README.md, frameworkcharlie.json, go.mod, charlie.go, charlie_test.go

### Nuevo: Framework Excel

- **Framework para conectar, leer y escribir archivos Excel**
- Soporte para lectura de archivos Excel completos
- Soporte para leer hojas específicas
- Acceso a valores de celdas individuales
- Cliente con tracing integrado (Paladin)
- Estructura modular: cmd/, internal/, temp/

### Nuevo: Framework Quine

- **Framework de quines auto-replicantes** para el proyecto Remora
- Sistema de revisión de código
- Integración con Paladin para tracing
- AGENTS.md e INITIAL_PROMPT.md para guías de uso
- Estructura: cmd/, internal/quine/, internal/review/, internal/types/

### Expansión: Framework Paladin

- **SYSTEM.md**: Nuevo prompt del sistema con documentación completa
- **docs/MERE.md**: Documentación de la estructura MERE
- **examples/**: Dos ejemplos nuevos:
  - `01_basic/`: Uso básico de tracing
  - `02_decisions/`: Ejemplo de decisiones lógicas con contexto

### Expansión: Framework Echo

- **cmd/framework-echo/**: Nuevo ejecutable principal
- **internal/paladin/**: Módulo de tracing integrado en Echo
  - console.go, context.go, span.go, trace.go
- **docs/SYSTEM_PROMPT.md**: Actualizado con nuevas instrucciones

---

## [0.1.1] - 2026-04-25

> **Important**: This release generalizes the MERE data model to work across any business domain, replacing domain-specific entities (payments, invoices, etc.) with generic patterns.

### Framework Alfa

#### Breaking Changes

- **Generic MERE Entities**: Replaced domain-specific entities with domain-agnostic patterns:
  - Removed: `recurso_recibido`, `planilla_actual`, `documento_comercial_actual`, `movimiento_de_dinero_actual`, `contraparte`, `documento_comercial`, `pago`, `aplicacion_pago`
  - Added: `artefacto_actual`, `registro_actual`, `actor_actual`, `objeto_operativo_actual` (current state)
  - Added: `actor`, `entidad_negocio`, `evento_operativo`, `relacion_normalizada`, `evidencia`, `estado_historial` (normalized target)

- **Generic Cardinality Questions**: Replaced "pagos parciales" question with:
  > "Cuando relacionan esos elementos, ¿la relación es siempre 1 a 1, puede ser 1 a muchos, muchos a muchos, parcial o con excepciones?"

#### New Features

- **Domain-Agnostic Data Model**: Alfa now compiles generic MERE structures that can apply to any business:
  - `actor`: Person, organization, area or system with a defined role
  - `entidad_negocio`: Main thing the business needs to track, classify or decide about (name not fixed without Echo evidence)
  - `evento_operativo`: Event that changes state, history, amount, priority, responsible or decision on an entity
  - `relacion_normalizada`: Generic associative entity for crosses between two or more elements when cardinality is unconfirmed
  - `evidencia`: Original artifact that backs structured data and enables auditing
  - `estado_historial`: History of states or stages when process depends on temporal tracking

- **Advanced Gap Detection Functions**:
  - `evidenceLikely()`: Detects unstructured resources (whatsapp, capture, image, photo, receipt, email, pdf, file, paper, message)
  - `relationshipLikely()`: Detects relationship needs (cruzar, cruce, relacionar, associate, calzar, conciliar, match)
  - `identifierGapLikely()`: Detects potential duplicate issues
  - `cardinalityConfirmed()`: Checks if cardinality (1:1, 1:N, N:M, partial, etc.) was explicitly mentioned

- **Generic Business Rules**:
  - `data_rule_001`: Don't invent domain entities (Alfa can propose generic MERE but cannot fix names, fields or rules without Echo evidence)
  - `data_rule_002`: Don't assume cardinality (when flow relates elements, Alfa cannot assume 1:1, 1:N or N:M without confirmation)
  - `data_rule_003`: Preserve original evidence (automation must maintain link between normalized data and original resource)

- **Updated Compilation Logic**:
  - `normalizedEntities()`: Builds generic entity structure from conversation patterns
  - `normalizedRelationships()`: Builds relationship model with cardinality_unconfirmed flags
  - `dataModelGaps()`: Generates open questions based on detected gaps in relationships, identifiers and evidence context

- **MERE Verbalization for Bravo**: Export now includes full generic MERE structure with entities, relationships and business rules

#### Improvements

- `INITIAL_PROMPT.md`: Updated MERE section to emphasize generic patterns over domain-specific examples
- `compile.go`: Refactored with helper functions for cleaner, more maintainable code
- `compile_test.go`: Updated tests to use generic relationship patterns instead of payment-specific scenarios

### Framework Echo

- **Enhanced Resource Detection**: `readiness.go` now detects more evidence sources (message, chat, email, pdf, file, document, paper)
- **Generic Context Commitment**: Updated wording to handle any resource-to-record linking, not just payments/invoices
  - Before: "Para automatizar esto necesito unir transferencia, factura y cliente..."
  - After: "Para automatizar esto necesito unir cada recurso con el registro correcto..."
- **Updated Prompts**: AGENTS.md, INITIAL_PROMPT.md, SYSTEM_PROMPT.md all refined with generic patterns

### Documentation

- `nuevo_mapa.md`: Added clarification that cardinality rules are general, not specific to payments:
  > "La regla general no es 'pagos y facturas'. La regla general es: cuando la automatización necesita relacionar elementos, Alfa debe saber si la relación es 1 a 1, 1 a muchos, muchos a muchos, parcial, temporal o con excepciones."

---

## [0.1.0] - 2026-04-24

### Initial Release

#### Frameworks

- **Framework Alfa**: Compilation engine for translating Echo's validated intent into verifiable flow specs for Bravo
- **Framework Bravo**: Flow verification and validation framework
- **Framework Echo**: Discovery and opportunity validation framework
- **Remora Flujo**: Main orchestration layer

#### Core Capabilities

- Tree-based opportunity validation (OPPORTUNITY -> PAIN -> TASK -> THEORY -> AXIOM lineage)
- Draft compilation for early-stage opportunities
- Readiness-based compilation gates
- Export-ready validation before Bravo handoff
- Open questions tracking for incomplete information