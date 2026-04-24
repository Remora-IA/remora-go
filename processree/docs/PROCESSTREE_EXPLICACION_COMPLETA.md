# PROCESSTREE - Documentación para Agentes IA

## ¿Qué es?

Processree es un framework CLI en Go para hacer **reverse engineering de procesos de negocio**. Suena técnico pero en la práctica es simple:

**El objetivo:** Descubrir qué tareas son repetitivas y causarían valor automatizarse.

**El método:** Construir un árbol de conocimiento mediante conversación validada.

**La mejora clave:** La IA no solo registra hechos. También guarda percepciones internas sobre comportamiento, contradicciones y dolores no verbalizados. Eso evita saltar a "una app" o "un bot" antes de entender el dolor real.

---

## Concepto clave: El usuario NO sabe qué automatizar

El dueño de la empresa cree que sabe qué necesita ("quiero una app"). Pero en realidad no conoce las opciones ni el impacto real. Por eso:

- El cliente CONFIRMA problemas (PAINS)
- La IA ANOTA oportunidades candidatas después de PAINS confirmados
- La IA RECOMIENDA solo cuando la oportunidad encaja con el dolor real
- El cliente confirma si la recomendación resuelve el problema, pero no elige entre opciones técnicas

---

## Capas del Árbol

```
┌─────────────────────────────────────────────────────────────┐
│                         PAINS (Layer 3)                     │
│  Problemas/fricciones que el cliente confirmó               │
│  Ejemplo: "El cliente se frustra esperando respuesta"       │
├─────────────────────────────────────────────────────────────┤
│                         TASKS (Layer 2)                     │
│  Tareas repetitivas descubiertas                            │
│  Ejemplo: "Consultar estado de marca manualmente"          │
├─────────────────────────────────────────────────────────────┤
│                      THEORIES (Layer 1)                     │
│  Hipótesis que la IA infiere (pendiente confirmar)          │
│  Ejemplo: "El cliente consulta 1-2 veces por semana"      │
├─────────────────────────────────────────────────────────────┤
│                       AXIOMS (Layer 0)                      │
│  Hechos que el cliente confirma                             │
│  Ejemplo: "No existe base de datos, solo correos"           │
└─────────────────────────────────────────────────────────────┘

Después de PAINS existe una capa adicional:

┌─────────────────────────────────────────────────────────────┐
│                  OPPORTUNITIES (Layer 4)                    │
│  Automatizaciones candidatas anotadas, no ofrecidas aún     │
│  Ejemplo: "Base simple de clientes ordenada"                │
└─────────────────────────────────────────────────────────────┘
```

## Reglas de desbloqueo

- Layer 1 (THEORIES) → necesita 3 AXIOMS validados
- Layer 2 (TASKS) → necesita 3 THEORIES validadas  
- Layer 3 (PAINS) → necesita 2 TASKS validadas
- Layer 4 (OPPORTUNITIES) → necesita 1 PAIN validado

---

## Comandos disponibles

```bash
# Inicializar proyecto
./processtree init --project-id "registro-marcas" --client "Gamma" --date "2026-04-23"

# Crear nodos por capa
./processtree add-axiom --title "No existe base de datos" --evidence "Solo correos"
./processtree add-theory --parent ax_001 --title "Consultan seguido" --evidence "Hipotesis"
./processtree add-task --parent th_001 --title "Consultar manualmente" --evidence "Confirmado"
./processtree add-pain --parent tk_001 --title "Tiempo perdido" --evidence "Confirmado"
./processtree add-opportunity --parent pn_001 --title "Base simple de clientes" --evidence "Candidata para resolver búsqueda en libreta"
./processtree add-perception ax_001 --note "El cliente dice 'no tengo idea': no puede diseñar solución, conviene preguntar por conducta actual"

# Validar/rechazar nodos
./processtree validate th_001 --answer "Si, lo hacen每周"
./processtree reject th_002 --reason "No aplica"

# Consultar estado
./processtree show-tree    # Ver árbol completo
./processtree status       # Ver estadísticas
./processtree next-questions  # Ver preguntas pendientes
```

---

## Ejemplo de FLUJO REAL: Reunión con Cliente

Este es exactamente cómo funciona una sesión real:

---

### Contexto

El agente (PI) está guiando a "Usuario" que está en una reunión con su cliente "Gamma" (una empresa de registro de marcas).

---

### CHAT REAL: Usuario ↔ PI

**Usuario:** "Usemos processree"

**PI:** "Perfecto. Para entender el proceso, podrías preguntarle: **¿Cuál es la actividad que más tiempo les toma?**"

---

**Usuario va a la reunión y le pregunta al cliente de Gamma:**
"¿Cuál es la actividad que más tiempo les toma?"

**Cliente de Gamma:** "Las consultas de los clientes preguntando por el estado de sus marcas"

**Usuario vuelve y le dice a PI:**

**Usuario:** "Las consultas de estado de marcas"

**PI:** ✅ Crea AXIOM ax_001: "Consultas de estado de marca es actividad frecuente"
   Sugiere siguiente pregunta: "¿Cuántas consultas hacen al día/semana?"

---

**Usuario pregunta al cliente:**
"¿Cuántas consultas hacen al día?"

**Cliente:** "Depende, a veces 3, a veces 10, pero mínimo unas 15 a la semana"

**Usuario:** "15 a la semana"

**PI:** ✅ Crea AXIOM ax_002: "Mínimo 15 consultas semanales"
   Sugiere: "¿Cómo responden esas consultas?"

---

**Usuario pregunta:**
"¿Y cómo responden esas consultas?"

**Cliente:** "Por WhatsApp principalmente. Vemos el correo de INAPI y les respondemos"

**Usuario:** "Por WhatsApp, verifican correo de INAPI y responden"

**PI:** ✅ Crea AXIOM ax_003: "Proceso manual: WhatsApp + verificación en correo de INAPI"

---

**Usuario pregunta:**
"¿Cuánto tiempo les toma responder cada consulta?"

**Cliente:** "Depende, si tenemos el correo a mano son 5 minutos, pero a veces nos toca buscar en el historial y se va a 20 minutos"

**Usuario:** "5 a 20 minutos según disponibilidad"

**PI:** ✅ Crea AXIOM ax_004: "Tiempo variable: 5-20 minutos por consulta"
   Sugiere: "¿Los clientes se frustran esperando respuesta?"

---

**Usuario pregunta:**
"¿Los clientes se frustran esperando la respuesta?"

**Cliente:** "Sí, a veces llaman varias veces al día preguntando lo mismo"

**Usuario:** "Sí, llaman varias veces al día preguntando lo mismo"

**PI:** ✅ Crea AXIOM ax_005: "Clientes insatisfechos: llaman varias veces"

---

**PI:** "Ya tienes 5 axiomas. Puedo desbloquear Layer 1 (THEORIES)"

---

### Avanzando a Layer 1: THEORIES

**PI:** "Basado en los axiomas, infiero estas teorías. Pregúntale al cliente para confirmar:"

- **TH_001:** "El cliente podría tener información más rápida si consultara directo a INAPI"
  Pregunta: "¿Si pudieran consultar el estado directo en INAPI, cuánto tiempo ahorrarían?"

- **TH_002:** "Los clientes podrían auto-consultar si tuvieran acceso"
  Pregunta: "¿Estarían dispuestos los clientes a consultar el estado ellos mismos?"

---

**El árbol se ve así:**

```
                    ax_001: Consultas frecuentes
                         │
                    ax_002: 15/semana mínimo
                         │
                    ax_003: Proceso manual (WhatsApp + email)
                         │
                    ax_004: 5-20 min por consulta
                         │
                    ax_005: Clientes insatisfechos
                              │
                         [THEORIES]
                              │
                         th_001: Acceso directo a INAPI
                         th_002: Clientes auto-consultan
```

---

## Cómo PI decide las PREGUNTAS

PI no inventa preguntas al azar. Sigue esta lógica:

| Tipo de Axiom | Siguiente pregunta |
|---------------|---------------------|
| Menciona actividad | "¿Cuántas veces?" |
| Menciona tiempo | "¿Cuánto tarda?" |
| Menciona espera | "¿Quién espera?" |
| Menciona repetición | "¿Eso pasa seguido?" |
| Menciona error/frustración | "¿Cada cuánto?" |

La tabla es una guía, no un piloto automático. La IA debe preguntar lo necesario para aclarar el camino. Si una pregunta no ayuda a encontrar el dolor real, está de más.

### Preguntas que funcionaron en pruebas

Las mejores preguntas fueron concretas y fáciles de responder:

- "¿Qué es lo que más tiempo te toma buscar?"
- "Cuando no tienes tu PC a mano, ¿cómo respondes hoy?"
- "¿Ha pasado que se te olvida responder o se te mezclan los casos?"
- "¿Qué te pasa cuando tienes que responder un correo?"
- "¿Dónde tienes esa información ahora?"

Estas preguntas revelan comportamiento real. No le piden al cliente imaginar una solución.

### Percepciones internas

Cuando una respuesta revela algo más profundo, se guarda como percepción:

```bash
./processtree add-perception ax_021 --note "El cliente llama 'desastre' a la libreta: hay dolor emocional y desorden operativo, no solo tiempo perdido"
```

Ejemplos de percepciones:

- "No tengo idea" → el cliente no puede formular soluciones; hay que volver a conducta actual.
- "Es un cacho" → hay dolor emocional; conviene profundizar ahí.
- Corrige números → estima sobre la marcha, no mide.
- Propone "app" o "WhatsApp" → puede estar nombrando lo conocido, no lo necesario.
- Dice que algo es rápido pero luego describe 30 min → contradicción que revela fricción oculta.

---

## Cómo PI genera THEORIES

De los axiomas, PI infiere patrones:

```
ax_003: Proceso manual (WhatsApp + email)
ax_004: 5-20 min por consulta
ax_005: Clientes insatisfechos
         ↓
    THEORY: "Si el cliente pudiera consultar directo, 
             no dependería del equipo"
         ↓
    Pregunta: "¿Si pudieran consultar el estado solos?"
```

---

## IMPORTANTE: Qué NO hace la IA

❌ NO pregunta "¿qué automatizar?"  
❌ NO sugiere soluciones sin haber encontrado PAINS  
❌ NO pide al cliente que elija entre opciones  
❌ NO crea nodos sin respuesta del cliente  
❌ NO ofrece una automatización apenas se le ocurre  
❌ NO obliga al usuario a adaptarse a la solución  

✅ SI pregunta sobre el PROCESO  
✅ SI infiere patrones  
✅ SI genera preguntas una por una  
✅ SI guarda percepciones internas cuando ayudan  
✅ SI anota OPPORTUNITIES después de PAINS confirmados  
✅ SI recomienda SOLO después de PAINS confirmados  

---

## Siguiente paso: ANOTAR OPORTUNIDADES

Cuando hay PAINS validados, PI debería anotar oportunidades candidatas:

```bash
./processtree add-opportunity --parent pn_001 --title "Base simple de clientes" --evidence "Resolvería búsqueda en libreta desordenada"
./processtree add-opportunity --parent pn_001 --title "Plantillas protocolares" --evidence "Resolvería dificultad de formular correos formales"
```

Anotar no es ofrecer.

Antes de recomendar:

1. Verifica que la oportunidad resuelva un PAIN real.
2. Verifica que encaje con la forma actual de trabajar.
3. Descarta soluciones que obligan al usuario a adaptarse demasiado.
4. Descarta software si hay una solución más simple.
5. Recién ahí recomienda una opción.

Ejemplo:

- PAIN: "Registro toma 30 min por buscar datos en libreta desordenada"
- OPPORTUNITY: "Base simple de clientes"
- NO recomendar todavía si no sabes si el usuario puede usar una interfaz nueva.
- Pregunta mejor: "Cuando te llama un cliente, ¿dónde tienes la libreta? ¿Está siempre a mano?"

---

## Visualización del GRAFO resultante

Este es el grafo que se construiría con la conversación de arriba:

```
═══════════════════════════════════════════════════════════════════
                         PROCESSREE: Gamma
═══════════════════════════════════════════════════════════════════

LAYER 0: AXIOMS ( hechos confirmados )
│
├── ax_001: "Consultas de estado de marca es actividad frecuente"
│   Evidence: "El cliente lo mencionó como principal actividad"
│   Status: ✅ VALIDATED (100%)
│
├── ax_002: "Mínimo 15 consultas semanales"
│   Evidence: "Cliente confirmó rango de 3-10 diario"
│   Status: ✅ VALIDATED (100%)
│
├── ax_003: "Proceso manual: WhatsApp + verificación en email"
│   Evidence: "Revisan correo de INAPI manualmente"
│   Status: ✅ VALIDATED (100%)
│
├── ax_004: "Tiempo variable: 5-20 minutos por consulta"
│   Evidence: "Depende de si tienen el correo a mano"
│   Status: ✅ VALIDATED (100%)
│
└── ax_005: "Clientes insatisfechos: llaman varias veces"
    Evidence: "Cliente reporta frustración del equipo"
    Status: ✅ VALIDATED (100%)

                    ↓ 3+ axiomas = puede subir a LAYER 1

LAYER 1: THEORIES ( hipótesis )
│
├── th_001: "Acceso directo a INAPI reduciría tiempo"
│   Evidence: "Proceso actual es 5-20 min"
│   Parent: ax_003, ax_004
│   Question: "¿Si pudieran consultar el estado directo a INAPI?"
│   Status: ⏳ PENDING
│
└── th_002: "Clientes podrían auto-consultar"
    Evidence: "Ya usan WhatsApp, insatisfechos"
    Parent: ax_003, ax_005
    Question: "¿Estarían dispuestos los clientes a consultar solos?"
    Status: ⏳ PENDING

                    ↓ 3+ theories validadas = LAYER 2

LAYER 2: TASKS ( tareas repetitivas )
│
├── tk_001: "Responder consultas de estado por WhatsApp"
│   Evidence: "15+ semanales, proceso manual"
│   Parent: ax_001, ax_003
│   Status: 🔒 LOCKED (necesita 3 theories validadas)

LAYER 3: PAINS ( problemas )
│
├── pn_001: "Equipo pierde tiempo en consultas que podrían ser automáticas"
│   Evidence: "15 consultas/semana × 10 min promedio = 2.5 horas"
│   Parent: ax_004, th_001
│   Status: 🔒 LOCKED (necesita 2 tasks validadas)

═══════════════════════════════════════════════════════════════════

ESTADÍSTICAS:
  Total nodos: 7
  Layer 0: 5 axiomas ✅
  Layer 1: 2 teorías ⏳
  Layer 2: 1 tarea 🔒
  Layer 3: 1 dolor 🔒

ESTADO: Puede crear THEORIES. Necesita validar th_001 y th_002 para avanzar.

═══════════════════════════════════════════════════════════════════
```

---

## Comandos para ver el grafo

```bash
./processtree show-tree    # Muestra el árbol completo
./processtree status       # Muestra estadísticas
./processtree next-questions  # Muestra preguntas pendientes
```

---

## Flujo resumido

```
REUNIÓN REAL
    │
    ▼
┌─────────────────┐
│ PI sugiere      │  "Pregunta: ¿Cuál es la actividad que más tiempo ocupa?"
│ pregunta        │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Usuario pregunta │  (en la reunión con el cliente)
│ al cliente      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Cliente responde│  "Las consultas de estado de marcas"
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Usuario reporta │  "Consultas de estado de marcas"
│ respuesta       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ PI crea AXIOM   │  (guarda en JSON, no toca el JSON manualmente)
│                 │  ./processtree add-axiom --title "..." --evidence "..."
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ PI sugiere      │  "¿Cuántas veces al día?"
│ siguiente        │
│ pregunta         │
└────────┬────────┘
         │
         ▼
      REPETIR hasta completar el árbol
```

---

## Notas importantes

1. **PI nunca edita JSON directamente** - siempre usa comandos
2. **El cliente confirma hechos, no soluciones**
3. **PI propone preguntas una a una**
4. **El árbol se construye con respuestas reales**
5. **Las capas se desbloquean con validaciones**

---

¿Entendido? Si tienes dudas sobre cómo funciona el flujo o la construcción del grafo, pregunta.
