# CLAUDE.md — remora-go-lite

## Qué es Rémora

Leer `WHY.md` primero. Es la fuente de verdad sobre el producto.

Rémora es una plataforma de automatización adaptativa. Un swarm de agentes
de IA especializados y autónomos que entienden, analizan y ejecutan procesos
complejos de negocio — con auditoría en tiempo real sobre ERPs, APIs y datos.

**Rémora es la plataforma. Cobranza es un perfil de ejemplo. Lau-ai es un
cliente separado (docs en `/Users/alcless_a1234_cursor/Reapps/remora/lau-ai-research/`).**

Este repo (`remora-go-lite`) es la implementación en Go de la plataforma.
Orquesta frameworks independientes compuestos declarativamente desde
`flow.rules.json` sobre un canal JSON-RPC.

---

## Estructura clave

- `WHY.md` — Por qué existe Rémora (Golden Circle)
- `ARCHITECTURE.md` — Modelo de composición de frameworks
- `framework-*/` — Cada capability es un binario autónomo
- `remora-flujo/` — Orquestador REST + frontend
- `profiles/<cliente>/` — Configuración y datos por cliente (ej: `cobranza-chile`)
- `channel/` — Ejecutor JSON-RPC + vault + axiomas de seguridad

## Reglas específicas de este repo

- Stack: Go. Los frameworks se comunican via JSON-RPC
- Cada `framework-*/` es una capability encapsulada — no modificar sin entender el contrato del canal
- Variables de entorno requeridas: `GROQ_API_KEY` o `MINIMAX_API_KEY`, `REMORA_VAULT_KEY`
- Vocabulario de dominio específico (ej: "deudor", "cobranza") debe vivir en perfiles, no en código core

## Antes de implementar cualquier cosa

1. Leer `WHY.md` para entender qué es el producto
2. Leer `ARCHITECTURE.md` para entender el modelo de composición de frameworks
3. Seguir el protocolo del CLAUDE.md raíz (`/Users/alcless_a1234_cursor/Reapps/remora/CLAUDE.md`)
