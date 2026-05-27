# Remora Doc-Swarm — Reporte de Benchmark

**Fecha:** 2026-05-27 06:25:03  
**Swarm ID:** `doc-swarm-1779863103`

## Métricas

| Métrica | Valor |
|---|---|
| Duración total | 53ms |
| Agentes | 3 |
| Zonas totales | 5 |
| Resueltas | 5 (100%) |
| Bloqueadas | 0 |
| Tasa de colisión | 0.0% |

## Resultados por zona

| Zona | Agente | Estado | Output | Tiempo |
|---|---|---|---|---|
| bravo | `agent-beta` | ✅ solved | 17 funcs exported, 10 types, 0 interfaces, 340 líneas | 2ms |
| paladin | `agent-gamma` | ✅ solved | 31 funcs exported, 12 types, 0 interfaces, 1085 líneas | 3ms |
| echo | `agent-alpha` | ✅ solved | 25 funcs exported, 10 types, 0 interfaces, 1177 líneas | 4ms |
| alfa | `agent-beta` | ✅ solved | 6 funcs exported, 17 types, 0 interfaces, 1244 líneas | 4ms |
| charlie | `agent-gamma` | ✅ solved | 54 funcs exported, 11 types, 0 interfaces, 2638 líneas | 5ms |

## Campo de presión final

| Zona | Presión | Densidad | Resuelta |
|---|---|---|---|
| framework-paladin | 0.000 | 1 | sí |
| framework-echo (tree) | 0.000 | 1 | sí |
| framework-bravo | 0.000 | 1 | sí |
| framework-alfa | 0.000 | 1 | sí |
| framework-charlie | 0.000 | 1 | sí |

## Feromonas (10 señales)

| Señal | Zona | Agente | Fuerza | Expiración |
|---|---|---|---|---|
| exploring | paladin | `agent-gamma` | 1.00 | 06:35:03 |
| exploring | echo | `agent-alpha` | 1.00 | 06:35:03 |
| exploring | bravo | `agent-beta` | 1.00 | 06:35:03 |
| solved | bravo | `agent-beta` | 1.00 | permanente |
| solved | paladin | `agent-gamma` | 1.00 | permanente |
| solved | echo | `agent-alpha` | 1.00 | permanente |
| exploring | alfa | `agent-beta` | 1.00 | 06:35:03 |
| exploring | charlie | `agent-gamma` | 1.00 | 06:35:03 |
| solved | alfa | `agent-beta` | 1.00 | permanente |
| solved | charlie | `agent-gamma` | 1.00 | permanente |

---
_Remora Doc-Swarm · github.com/remora-ia/remora-go_
