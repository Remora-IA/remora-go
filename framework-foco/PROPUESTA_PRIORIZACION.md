# Propuesta: Sistema de Priorización para Foco v3

## Principios Clave

1. **Foco es autónomo** - clasifica, reordena y mueve tareas automáticamente
2. **Preguntas Socráticas** - ayudan al usuario a encontrar la respuesta, no preguntan directamente
3. **Mantiene el WHY** - siempre enfoca en el resultado deseado
4. **Ofrece opciones** - nunca cierra caminos

---

## 1. Matriz de Eisenhower

```
                    URGENTE                  NO URGENTE
              ┌─────────────────────┬─────────────────────┐
   IMPORTANTE │      Q1: DO NOW     │      Q2: SCHEDULE    │
              │   Crisis, deadlines  │   Planning, prevention│
              ├─────────────────────┼─────────────────────┤
NO IMPORTANTE │      Q3: DELEGATE   │      Q4: ELIMINATE   │
              │  Interruptions     │      Time wasters    │
              └─────────────────────┴─────────────────────┘
```

---

## 2. Conceptos Separados

```
┌─────────────────────────────────────────────────────────┐
│ PRE-CONFLICTO = Problema que hay que arreglar           │
│   go run ./cmd/foco conflict --text "Falta credentials"│
│                                                         │
│ DEPENDENCIA = Secuencia (orden natural)                 │
│   go run ./cmd/foco depends --on task_A --task task_B  │
└─────────────────────────────────────────────────────────┘
```

---

## 3. Filosofía de Preguntas

### NO preguntas directas

```
❌ "¿Es importante esta tarea?"
❌ "¿Es urgente?"
❌ "¿Tiene deadline?"
❌ "¿Puedes hacerla hoy?"
```

### SÍ preguntas Socráticas (mantienen el WHY)

```
✓ "¿Si no pudieras hacer [tarea], tendrías otra forma de lograr [WHY]?"
✓ "Si no resuelves [pre-conflicto], ¿qué otra cosa podrías hacer?"
✓ "Si no tienes [recurso], ¿cómo podrías avanzar?"
✓ "Si no hicieras [tarea A], ¿podrías hacer [tarea B]?"
✓ "¿Qué pasaría si [X] no estuviera? ¿Se cumpliría el [WHY]?"
```

### Estructura de Preguntas Socráticas

```
1. CONDICIONAL: "Si no [evento A]..."
2. OPCIÓN: "¿[podrías / tendrías] [opción B]?"
3. PARA: "Para [resultado/WHY]?"

Ejemplo:
"Si no resuelves las credenciales de GCP, 
 ¿tendrías otra forma de hacer el deploy?
 Para [el demo de Charlie]."
```

---

## 4. Lógica Autónoma de Foco

### Reglas de Clasificación Automática

```
╔═══════════════════════════════════════════════════════════════╗
║                  REGLAS DE CLASIFICACIÓN                         ║
╠═══════════════════════════════════════════════════════════════╣
║                                                               ║
║  CONDICIÓN                          →  CUADRANTE               ║
║  ────────────────────────────────────────────────────────────  ║
║                                                               ║
║  due_date = hoy                      →  Q1 (urgente)         ║
║  due_date <= 24h                      →  Q1 (urgente)         ║
║  due_date <= 3 días                   →  Q1 o Q2              ║
║  due_date > 1 semana                  →  Q2 (puede esperar)  ║
║  due_date = null                       →  Q2 (sin deadline)    ║
║                                                               ║
║  "para Charlie" en evidencia           →  importance = high    ║
║  "demo" en título                       →  urgency = high       ║
║  "urgente" en título                    →  urgency = high       ║
║                                                               ║
║  has pre_conflicto = true             →  ESPERA               ║
║  has dependency unmet = true          →  ESPERA               ║
║                                                               ║
║  dependency_of_task_in_Q1 = true     →  importance = high      ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
```

---

## 5. Comandos Propuestos

```bash
# Pre-Conflicto
go run ./cmd/foco conflict --text "Falta credentials de GCP"
go run ./cmd/foco conflicts
go run ./cmd/foco resolve --id pc_001

# Dependencia
go run ./cmd/foco depends --on task_001 --task task_002
go run ./cmd/foco depends --on task_001   # qué depende de task_001
go run ./cmd/foco depends --for task_002   # de qué depende task_002

# Priorización
go run ./cmd/foco priority             # ver y auto-ordenar
go run ./cmd/foco priority --reorder   # reordenar según reglas

# Flujo y What-If
go run ./cmd/foco flow
go run ./cmd/foco whatif --task task_001
```

---

## 6. Simulaciones Realistas v3

### Simulación 1: Foco Clasifica Automáticamente

```
Foco: go run ./cmd/foco priority

Output:
  PRIORIDADES - 2026-04-28
  ═══════════════════════

  [AUTO] Clasificando 5 tareas...
  
  Q1 - DO NOW
  ┌────────────────────────────────────────┐
  │ task_003: Demo para Charlie           │
  │ Due: 2026-04-29 (mañana)             │
  │ Clasificado: evidence="Para Charlie"   │
  └────────────────────────────────────────┘
  
  Q2 - SCHEDULE
  ┌────────────────────────────────────────┐
  │ task_001: Implementar auth             │
  │ task_002: Escribir tests               │
  └────────────────────────────────────────┘
  
  ESPERA
  ┌────────────────────────────────────────┐
  │ task_deploy: Deploy a Cloud            │
  │ Pre-conflicto: pc_001 (credenciales)   │
  └────────────────────────────────────────┘

Foco: "1 tarea en Q1 (Charlie), 2 en Q2, 1 en espera.
       AHORA TOCA: task_003"
```

---

### Simulación 2: Foco Pregunta con Enfoque en el WHY

**Escenario:** Hay ambigüedad, Foco pregunta de forma Socrática.

```
Foco: go run ./cmd/foco flow

Output:
  FLUJO ACTUAL
  ─────────────────
  task_001 ──→ task_002 ──→ task_003

Foco detecta análisis semántico:
  - task_003 es "deploy a producción"
  - task_003 no tiene evidencia de importancia
  - Foco no está seguro de su prioridad

Foco: "UNA PREGUNTA para mantener el WHY:
  ────────────────────────────────────────
  Si no pudieras hacer el deploy a producción (task_003),
  ¿tendrías otra forma de lograr que los usuarios prueben el sistema?
  
  El WHY de hoy: 'Los usuarios pueden probar el sistema'"

User: "No, necesito hacer el deploy"

Foco: [AUTO] Clasificando...
  
  task_003 → Q1 (crítico para el WHY)

Foco: "Entendido. task_003 ahora es Q1.
       Flujo actualizado."
```

---

### Simulación 3: Foco Ofrece Alternativas Manteniendo el WHY

**Escenario:** Hay un pre-conflicto, Foco ofrece opciones para el WHY.

```
Foco: go run ./cmd/foco next

Output:
  AHORA TOCA: task_deploy
    id: task_deploy
    why: Deployar a Cloud Run para usuarios
    
INFO: Pre-conflicto activo: pc_001 (faltan credenciales)

Foco: "OPCIONES PARA EL WHY:
  ─────────────────────────────────────
  El WHY: 'Los usuarios pueden probar el sistema'
  
  task_deploy espera a: pc_001 (credenciales)
  
  Pero podrías lograr el mismo WHY de otras formas:
  
  ┌────────────────────────────────────────────┐
  │ Opción A: Deploy local                      │
  │   Los usuarios prueban en localhost          │
  │   ¿Es suficiente para hoy?                    │
  ├────────────────────────────────────────────┤
  │ Opción B: Usar cuenta compartida             │
  │   Otra persona ya tiene las credenciales     │
  │   ¿Quién podría compartir?                    │
  ├────────────────────────────────────────────┤
  │ Opción C: Continuar con task_auth            │
  │   Preparar todo para cuando estén las creds  │
  │   ¿Quieres avanzar en paralelo?              │
  └────────────────────────────────────────────┘

User: "Opción C - sigo con task_auth"

Foco: go run ./cmd/foco next
  
  AHORA TOCA: task_auth
    why: Preparar módulo auth para cuando esté el deploy"
```

---

### Simulación 4: Foco Detecta Dependencias Invisibles

**Escenario:** Foco detecta que falta una conexión lógica.

```
Foco: go run ./cmd/foco flow

Output:
  FLUJO ACTUAL
  ─────────────────
  task_tests
  task_deploy  ← keduanya tidak terlihat bergantung satu sama lain

Foco detecta análisis semántico：
  - task_tests prueba task_deploy
  - task_deploy no puede existir sin task_tests pasando

Foco: "OBSERVACIÓN para mantener el WHY:
  ────────────────────────────────────────
  Si no hicieras los tests (task_tests),
  ¿tendrías otra forma de asegurarte de que 
  el deploy funciona antes de que los usuarios lo vean?
  
  El WHY: 'Los usuarios usan un sistema que funciona'"

User: "No, necesito los tests"

Foco: go run ./cmd/foco depends --on task_tests --task task_deploy

Output:
  ✓ Dependencia agregada: task_deploy depende de task_tests

Foco: "Flujo actualizado.
       task_tests → task_deploy
       
       AHORA TOCA: task_tests"
```

---

### Simulación 5: Foco Reordena y Confirma

**Escenario:** Foco detecta que debe reordenar y avisa.

```
Foco: go run ./cmd/foco priority --reorder

Output:
  AVISO: Reordenando flujo
  ─────────────────────────
  Detecté cambios:
  
  1. task_003 ahora tiene due_date = HOY
     → Moviendo a Q1
  
  2. task_001 es prerrequisito de task_003
     → Moviendo a Q1 también
  
  Nuevo orden:
  task_001 (Q1) → task_003 (Q1) → task_002 (Q2)
  
  ¿Aplico los cambios?"
```

---

### Simulación 6: Foco Limpia Tareas Prescindibles

**Escenario:** Foco detecta tarea que no aporta al WHY.

```
Foco: go run ./cmd/foco priority

Output:
  Q4 - ELIMINATE
  ┌────────────────────────────────────────┐
  │ task_006: Investigar alternativa X     │
  │ Creado hace 3 semanas                 │
  │ Sin progreso                          │
  └────────────────────────────────────────┘

Foco: "PREGUNTA SOCRÁTICA:
  ────────────────────────────
  Si no hicieras la investigación de alternativa X,
  ¿afectaría eso el WHY del sprint?
  
  WHY: 'Sistema funcionando en producción'"

User: "No, no lo afecta"

Foco: "¿La elimino entonces?"

User: "Si"

Foco: go run ./cmd/foco done --id task_006 --evidence "No aporta al WHY"

Output:
  ✓ task_006 eliminada
```

---

### Simulación 7: Foco Identifica Qué Necesita del Usuario

**Escenario:** Foco necesita un recurso para clasificar.

```
Foco: go run ./cmd/foco priority

Output:
  Tarea sin clasificar:
  ┌────────────────────────────────────────┐
  │ task_007: Refactorizar módulo core    │
  │ Sin due_date, sin evidencia           │
  └────────────────────────────────────────┘

Foco: "PREGUNTA SOCRÁTICA:
  ────────────────────────────
  Si no refactorizaras el módulo core,
  ¿tendrías otra forma de lograr que el sistema sea mantenible?
  
  Si no, ¿cuál es el resultado que necesitas?
  'Mantener el sistema por los próximos 6 meses' o 'Otro'"

User: "El sistema mantenible por 6 meses"

Foco: [AUTO] Clasificando...
  
  task_007 → Q2 (importante para mantenibilidad)

Foco: "Listo. task_007 ahora es Q2."
```

---

## 7. Flujo de Decisión Autónomo con Preguntas Socráticas

```
┌─────────────────────────────────────────────────────────────────┐
│                  FOCO: PRIORIDAD AUTÓNOMA v3                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. CARGAR CONTEXTO                                            │
│     ├── Tareas, axiomas, fechas, evidencia                      │
│     └── WHY del día                                             │
│                                                                 │
│  2. CLASIFICAR AUTOMÁTICAMENTE                                   │
│     ├── due_date=hoy → Q1                                       │
│     ├── "Charlie" en evidencia → high importance              │
│     └── Con pre_conflicto → ESPERA                              │
│                                                                 │
│  3. SI HAY DUDA, PREGUNTA SOCRÁTICA                              │
│     └── "Si no [X], ¿tendrías otra forma de lograr [WHY]?"     │
│                                                                 │
│  4. INFORMAR ANTES DE CAMBIAR                                   │
│     └── "Detecté [cambio]. ¿Aplico?"                           │
│                                                                 │
│  5. ACTUAR                                                      │
│     └── Ejecutar cambios                                        │
│                                                                 │
│  6. MOSTRAR PRÓXIMA TAREA                                       │
│     └── "AHORA TOCA: [tarea]"                                   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 8. Catálogo de Preguntas Socráticas

### Para detectar importancia

```
❌ "¿Es importante?"
✓ "Si no hicieras [tarea], ¿qué pasaría con [WHY]?"
✓ "Si no resuelves [problema], ¿qué otra cosa podrías hacer para [resultado]?"
✓ "¿Qué pasaría si [X] no estuviera? ¿Se cumpliría [WHY]?"
```

### Para detectar urgencia

```
❌ "¿Es urgente?"
✓ "Si no haces [tarea] hoy, ¿qué deja de funcionar?"
✓ "Si no resuelves [X] ahora, ¿cuál es el impacto en [WHY]?"
```

### Para alternativas

```
✓ "Si no tienes [recurso], ¿cómo podrías avanzar hacia [WHY]?"
✓ "Si no resuelves [pre-conflicto], ¿tendrías otra opción para [resultado]?"
✓ "Si no pudieras hacer [A], ¿podrías hacer [B] para [WHY]?"
```

### Para mantener el foco

```
✓ "¿Esto acerca al WHY?"
✓ "Si [X] no contribuye al WHY, ¿tiene sentido hacerlo?"
```

---

## 9. Nota Importante: Lenguaje de Foco

> **ELIMINAR:**
> - ~~bloqueadores~~ → `flow` (flujo)
> - ~~bloqueado~~ → `espera`
> - ~~desbloquear~~ → `continuar`
>
> **PREGUNTAS:**
> - ~~"¿Es importante?"~~ → "Si no [X], ¿[resultado]?"
> - ~~"¿Es urgente?"~~ → "Si no [X] ahora, ¿qué pasa con [WHY]?"
>
> Foco NUNCA dice que algo está "bloqueado".
> Foco ofrece opciones para lograr el WHY.

---

## 10. Resumen de Decisiones

- [x] Foco clasifica automáticamente
- [x] Preguntas Socráticas (ayudan al usuario a encontrar respuesta)
- [x] Siempre enfoca en el WHY
- [x] Ofrece opciones, nunca cierra caminos
- [x] Informa antes de hacer cambios
- [x] Detecta dependencias invisibles

---

**¿Aceptas este diseño para implementar?**
