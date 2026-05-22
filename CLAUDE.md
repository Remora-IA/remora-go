# CLAUDE.md — remora-go-lite

Sistema multi-framework de IA en Go. Orquesta capabilities independientes
(echo, alfa, foco, sabio, mecanico, mensajero, hosting, etc.) compuestas
declarativamente desde `flow.rules.json` sobre un canal JSON-RPC.

Proyecto en fase de ideación/desarrollo — pensado para integrarse con
productos de lau como backend de agentes.

Protocolo cross-repo completo: `/Users/alcless_a1234_cursor/Reapps/remora/CLAUDE.md`

---

## Reglas específicas de este repo

- Stack: Go. Los frameworks son binarios independientes que se comunican via JSON-RPC
- Cada `framework-*/` es una capability encapsulada — no modificar sin entender el contrato del canal
- Variables de entorno requeridas: `GROQ_API_KEY` o `MINIMAX_API_KEY`, `REMORA_VAULT_KEY`
- No hay artefactos `.pi/` todavía — si se hace una implementación significativa, generarlos primero

## Antes de implementar cualquier cosa

1. Leer `ARCHITECTURE.md` para entender el modelo de composición de frameworks
2. Correr `pi pi-impacto` describiendo el cambio — este repo puede afectar productos de lau
3. Seguir el protocolo del CLAUDE.md raíz
