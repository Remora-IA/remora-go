# MVP: Pressure Fields para Radiografía de Abogado Contrario

## Contexto para la IA ejecutora

Eres un 10x engineer. Mañana hay una presentación para un estudio jurídico chileno. Necesitas demostrar que un swarm de agentes de IA puede analizar el historial completo de un abogado contrario y producir una "radiografía" táctica — patrones, tendencias, predicciones — usando coordinación autónoma por campos de presión (sin orquestador central).

El framework base es `pressure-field-experiment` de Rodriguez (2026), que demostró 48.5% solve rate vs 1.5% del enfoque jerárquico en 1,350 trials. Vas a adaptarlo al dominio legal.

---

## Paso 0: Clonar el repo y entender la arquitectura

```bash
git clone https://github.com/Govcraft/pressure-field-experiment.git
cd pressure-field-experiment
```

### Arquitectura del repo (NO modificar survival-kernel)

```
crates/
├── survival-kernel/     ← Framework genérico. NO TOCAR. Úsalo como dependencia.
│   └── src/
│       ├── artifact.rs      # Trait Artifact (domain-agnostic)
│       ├── pressure.rs      # Trait Sensor, Trait Pressure, measure_pressure_inline
│       ├── region.rs        # RegionView, Patch, PatchOp, RegionState
│       ├── kernel.rs        # AsyncKernelBuilder, half_life_decay, run loop
│       ├── config.rs        # KernelConfig, PressureAxisConfig, DecayConfig
│       ├── actors/
│       │   ├── coordinator.rs    # KernelCoordinator (tick loop)
│       │   ├── region_actor.rs   # RegionActor (owns region state)
│       │   ├── sensor_actor.rs   # SensorActor (wraps Sensor trait)
│       │   └── claim_manager.rs  # Stigmergic claims (prevents duplicate work)
│       └── messages.rs      # Message types between actors
│
└── schedule-experiment/ ← Dominio de scheduling. Este es tu EJEMPLO de referencia.
    └── src/
        ├── artifact.rs      # ScheduleArtifact (implementa Artifact trait)
        ├── sensors.rs       # GapSensor, OverlapSensor, etc.
        ├── llm_actor.rs     # Construye prompts, parsea respuestas del LLM
        ├── experiment.rs    # ExperimentRunner (wires everything)
        ├── main.rs          # CLI
        ├── example_bank.rs  # Few-shot examples (pheromone system)
        ├── conversation.rs  # Baseline comparisons
        ├── generator.rs     # Random problem generator
        └── vllm_client.rs   # HTTP client for OpenAI-compatible API
```

### Lo que debes crear

```
crates/
└── legal-experiment/    ← TU CRATE. Implementa el dominio legal.
    ├── Cargo.toml
    └── src/
        ├── main.rs              # CLI para correr el experimento
        ├── artifact.rs          # LawyerProfileArtifact (implementa Artifact trait)
        ├── sensors.rs           # CompletitudSensor, EvidenciaSensor, CoherenciaSensor
        ├── llm_actor.rs         # Prompts legales, parseo de respuestas
        ├── experiment.rs        # Wiring del experimento
        ├── mock_data.rs         # Datos mock realistas (ver sección abajo)
        └── pjud_validator.rs    # Validación contra API real del PJUD
```

Agregar al `Cargo.toml` del workspace:

```toml
[workspace]
members = ["crates/survival-kernel", "crates/schedule-experiment", "crates/legal-experiment"]
```

---

## Paso 1: Generar datos mock REALISTAS

### El escenario exacto (de LAU_AI_PRODUCTO.pdf)

> "La audiencia preparatoria de O-892-2026 es mañana. El agente analizó las últimas 23 audiencias del abogado contrario — Andrés Fuentes, Larraín & Cía — y detectó su patrón: siempre intenta limitar los puntos de prueba citando el Art. 453 N°2 CT."

### Datos a generar: 50 causas donde Andrés Fuentes es abogado

Cada causa DEBE tener estos campos (formato exacto del PJUD):

```json
{
  "rol": "O-892-2026",
  "fecha_ingreso": "15/01/2026",
  "tribunal": "2° Juzgado del Trabajo de Santiago",
  "juez": "Hernández Contreras, María José",
  "materia": "Despido injustificado",
  "caratulado": "GONZÁLEZ MUÑOZ PEDRO con ELÉCTRICA DEL NORTE S.A.",
  "estado": "En tramitación",
  "abogado_demandante": "Silva Rojas, Carolina",
  "abogado_demandado": "Fuentes Larraín, Andrés",
  "estudio_demandado": "Larraín & Cía",
  "monto_demandado": 45000000,
  "movimientos": [
    {
      "fecha": "15/01/2026",
      "descripcion": "Ingreso demanda",
      "folio": "1"
    },
    {
      "fecha": "22/01/2026",
      "descripcion": "Resolución provee demanda",
      "folio": "2"
    },
    {
      "fecha": "10/02/2026",
      "descripcion": "Contestación de demanda",
      "folio": "3"
    },
    {
      "fecha": "15/03/2026",
      "descripcion": "Audiencia preparatoria",
      "folio": "4",
      "detalle_audiencia": {
        "tipo": "preparatoria",
        "resultado": "Se fijan puntos de prueba",
        "incidencias": [
          {
            "tipo": "solicitud_limitacion_prueba",
            "solicitante": "demandado",
            "argumento": "Art. 453 N°2 CT - hechos no controvertidos",
            "resultado": "rechazada",
            "puntos_prueba_fijados": [
              "Existencia de relación laboral",
              "Causal de despido",
              "Monto de remuneraciones"
            ]
          }
        ]
      }
    }
  ],
  "resultado_final": {
    "tipo": "sentencia",
    "fecha": "20/08/2026",
    "resultado": "Acoge parcialmente",
    "monto_concedido": 32000000,
    "fundamentos_clave": "Se acredita despido injustificado. Art. 168 CT."
  }
}
```

### PATRONES QUE DEBEN ESTAR EMBEBIDOS EN LOS 50 CASOS (no inventar, representar)

Los patrones DEBEN ser descubribles por los agentes. No pueden ser obvios (eso sería trampa) pero deben ser estadísticamente significativos:

**Patrón 1 — Limitación de prueba (FRECUENCIA ALTA):**
- En 39 de 50 causas (78%), Fuentes solicita limitar puntos de prueba citando Art. 453 N°2 CT
- En las 11 restantes no lo hace (causas que se conciliaron antes de audiencia preparatoria)

**Patrón 2 — Resultado ante juez Hernández vs otros jueces:**
- Ante juez Hernández: Fuentes pierde la limitación 60% de las veces (14 de 23 causas)
- Ante otros jueces: Fuentes gana la limitación 55% de las veces (9 de 16 causas con otros jueces)
- Hay 11 causas sin audiencia preparatoria (conciliadas antes)

**Patrón 3 — Conciliación post-rechazo:**
- Cuando Hernández rechaza la limitación, Fuentes ofrece conciliación en la siguiente audiencia 64% de las veces (9 de 14)
- Rango de conciliación: 40-55% del monto demandado (promedio 47%)

**Patrón 4 — Tendencia temporal:**
- Antes de 2024: Fuentes conciliaba 60% de sus causas
- Desde 2024: Fuentes solo concilia 35% — cambió a estrategia más agresiva
- Correlación con cambio de socio en Larraín & Cía (no explícito en datos, pero el cambio de patrón sí)

**Patrón 5 — Artículos de ley preferidos:**
- Art. 453 N°2 CT: 39 causas (limitación de prueba)
- Art. 163 CT: 28 causas (causal de necesidades de la empresa)
- Art. 168 CT: 15 causas (indemnización por despido injustificado, cuando pierde)

### Distribución de las 50 causas

| Rango | Cantidad | Tribunal | Juez |
|-------|----------|----------|------|
| 2019-2020 | 8 causas | Varios laborales Santiago | Varios |
| 2021-2022 | 12 causas | Varios laborales Santiago | Varios, 5 ante Hernández |
| 2023-2024 | 15 causas | Mayormente 2° Jdo Trabajo | 10 ante Hernández |
| 2025-2026 | 15 causas | 2° Jdo Trabajo + otros | 8 ante Hernández |

### Materias (realistas para laboral Chile)

- Despido injustificado: 28 causas
- Despido indirecto (autodespido): 8 causas
- Tutela de derechos fundamentales: 5 causas
- Cobro de prestaciones laborales: 6 causas
- Accidente del trabajo: 3 causas

### Empresas que Fuentes representa (sector energía + construcción)

- Eléctrica del Norte S.A.: 8 causas
- CGE Distribución S.A.: 6 causas
- Colbún S.A.: 5 causas
- Constructora Sigdo Koppers: 7 causas
- Constructora Echeverría Izquierdo: 5 causas
- Empresas varias (una cada una): 19 causas

### Formato de nombres (PJUD real usa MAYÚSCULAS en caratulado)

```
"GONZÁLEZ MUÑOZ PEDRO con ELÉCTRICA DEL NORTE S.A."
"RAMÍREZ SOTO JUAN CARLOS con CGE DISTRIBUCIÓN S.A."
"CONTRERAS VEGA MARÍA ISABEL con COLBÚN S.A."
```

### Roles de causa (formato PJUD)

- Laboral: `O-{número}-{año}` (ej: O-892-2026, O-1234-2024)
- Los números van de 100 a 9999
- Los años van de 2019 a 2026

---

## Paso 2: Definir el Artifact — LawyerProfileArtifact

El "artefacto" que los agentes construyen colaborativamente es la RADIOGRAFÍA del abogado. Se divide en REGIONES (como el schedule se dividía en time blocks):

```rust
// Cada región es una sección de la radiografía que necesita ser completada
enum RegionKind {
    PerfilBasico,          // Nombre, estudio, años activo, volumen de causas
    HistorialCronologico,  // Timeline de actividad año por año
    PatronesTacticos,      // Tácticas recurrentes detectadas (el corazón)
    AnalisisJueces,        // Rendimiento ante cada juez
    PrediccionAudiencia,   // Qué va a hacer mañana en O-892-2026
    EstrategiaRecomendada, // Qué hacer para ganarle
}
```

### Estado inicial del artefacto (presión máxima — todo vacío)

```json
{
  "regions": [
    {
      "id": "perfil_basico",
      "kind": "PerfilBasico",
      "content": "",
      "metadata": {"fuentes_count": "50", "abogado": "Fuentes Larraín, Andrés"}
    },
    {
      "id": "historial",
      "kind": "HistorialCronologico",
      "content": "",
      "metadata": {}
    },
    {
      "id": "patrones",
      "kind": "PatronesTacticos",
      "content": "",
      "metadata": {}
    },
    {
      "id": "jueces",
      "kind": "AnalisisJueces",
      "content": "",
      "metadata": {}
    },
    {
      "id": "prediccion",
      "kind": "PrediccionAudiencia",
      "content": "",
      "metadata": {"causa_objetivo": "O-892-2026", "juez": "Hernández"}
    },
    {
      "id": "estrategia",
      "kind": "EstrategiaRecomendada",
      "content": "",
      "metadata": {}
    }
  ]
}
```

---

## Paso 3: Definir Sensores

Cada sensor mide una dimensión de calidad. Son funciones puras: `RegionView → Signals`.

```rust
// Sensor 1: ¿Qué tan completa está esta sección?
struct CompletitudSensor;
// Señales: {"completitud": 0.0..1.0}
// 0.0 = vacía, 1.0 = todos los campos presentes
// Para PerfilBasico: tiene nombre? estudio? años activo? volumen? tendencia?
// Para PatronesTacticos: tiene al menos 3 patrones con evidencia y confianza?

// Sensor 2: ¿La sección cita evidencia concreta?
struct EvidenciaSensor;
// Señales: {"evidencia_ratio": 0.0..1.0}
// Cuenta menciones de roles de causa específicos (O-XXX-YYYY)
// Un patrón sin citar causas concretas = 0.0
// Un patrón citando "en 39 de 50 causas (O-234-2023, O-567-2024...)" = 1.0

// Sensor 3: ¿Las secciones son coherentes entre sí?
struct CoherenciaSensor;
// Señales: {"coherencia": 0.0..1.0}
// La predicción debe ser coherente con los patrones
// La estrategia debe responder a la predicción
// Si predicción dice "va a pedir limitar prueba" pero estrategia no lo menciona = 0.0

// Sensor 4: ¿La sección tiene profundidad analítica?
struct ProfundidadSensor;
// Señales: {"profundidad": 0.0..1.0}
// PerfilBasico: menciona tendencia temporal? = más profundo
// PatronesTacticos: tiene confianza estadística? = más profundo
// PrediccionAudiencia: tiene probabilidades? = más profundo
```

### Ejes de presión (KernelConfig)

```rust
let pressure_axes = vec![
    PressureAxisConfig {
        name: "vaciedad".into(),
        weight: 1.0,
        expr: "completitud".into(),  // invertido: presión = 1.0 - completitud
        kind_weights: HashMap::new(),
    },
    PressureAxisConfig {
        name: "sin_evidencia".into(),
        weight: 0.8,
        expr: "evidencia_ratio".into(),
        kind_weights: HashMap::from([
            ("PatronesTacticos".into(), 1.2),  // más peso para patrones
            ("PerfilBasico".into(), 0.3),      // menos peso para perfil
        ]),
    },
    PressureAxisConfig {
        name: "incoherencia".into(),
        weight: 0.6,
        expr: "coherencia".into(),
        kind_weights: HashMap::from([
            ("PrediccionAudiencia".into(), 1.5), // la predicción DEBE ser coherente
            ("EstrategiaRecomendada".into(), 1.5),
        ]),
    },
    PressureAxisConfig {
        name: "superficialidad".into(),
        weight: 0.5,
        expr: "profundidad".into(),
        kind_weights: HashMap::new(),
    },
];
```

---

## Paso 4: Definir los Agentes (LLM Actors)

### Equipo RAZONAMIENTO (9 agentes, 3 sub-equipos de 3)

Cada agente es un `LlmActor` con un system prompt distinto. Todos compiten por reducir presión en las regiones. El kernel NO les asigna regiones — ellos eligen la de mayor presión que matchee su capacidad.

**Sub-equipo Comprensión:**

```
Agente 1: Lector de Contexto
- System prompt: "Eres un analista legal. Dado un conjunto de causas judiciales,
  lee y comprende el contexto general: quién es este abogado, a quién representa,
  en qué tribunales opera, cuál es su volumen."
- Regiones que puede parchear: PerfilBasico, HistorialCronologico
- Temperatura: 0.15 (explotación — datos factuales)

Agente 2: Constructor de Relaciones
- System prompt: "Eres un analista de redes. Dado un conjunto de causas, construye
  el mapa de relaciones: qué abogado aparece ante qué jueces, representa a qué
  empresas, en qué materias. Identifica clusters y concentraciones."
- Regiones que puede parchear: PerfilBasico, AnalisisJueces
- Temperatura: 0.25

Agente 3: Historiador
- System prompt: "Eres un cronólogo legal. Ordena la actividad cronológicamente,
  identifica períodos de cambio, tendencias de crecimiento o contracción, y
  cambios de comportamiento en el tiempo."
- Regiones que puede parchear: HistorialCronologico
- Temperatura: 0.2
```

**Sub-equipo Análisis:**

```
Agente 4: Buscador de Patrones
- System prompt: "Eres un detective de patrones legales. Busca comportamientos
  repetitivos: tácticas que este abogado usa consistentemente, argumentos que
  repite, estrategias recurrentes. Cada patrón debe tener frecuencia y confianza."
- Regiones que puede parchear: PatronesTacticos
- Temperatura: 0.35 (balanced — necesita creatividad para encontrar patrones)

Agente 5: Calculador Estadístico
- System prompt: "Eres un estadístico legal. Valida patrones con números:
  frecuencias, porcentajes, tasas de éxito. Si un patrón no tiene significancia
  estadística, márcalo como débil. Cita las causas exactas como evidencia."
- Regiones que puede parchear: PatronesTacticos, AnalisisJueces
- Temperatura: 0.15 (explotación — cálculos precisos)

Agente 6: Detector de Anomalías
- System prompt: "Eres un analista de anomalías. Busca lo que se sale del patrón:
  cambios repentinos de comportamiento, casos atípicos, resultados inesperados.
  Si el abogado cambió de estrategia en algún momento, encuentra cuándo y por qué."
- Regiones que puede parchear: PatronesTacticos, HistorialCronologico
- Temperatura: 0.45 (exploración — buscar lo inesperado)
```

**Sub-equipo Juicio:**

```
Agente 7: Comparador
- System prompt: "Eres un analista comparativo. Compara el rendimiento de este
  abogado ante distintos jueces, en distintas materias, en distintos períodos.
  Identifica dónde es fuerte y dónde es débil."
- Regiones que puede parchear: AnalisisJueces
- Temperatura: 0.3

Agente 8: Predictor
- System prompt: "Eres un predictor legal. Dado el historial de patrones, la
  información del juez, y el contexto de la causa objetivo, predice qué va a hacer
  este abogado en la próxima audiencia. Asigna probabilidades. Sé específico."
- Regiones que puede parchear: PrediccionAudiencia
- Temperatura: 0.25
- NOTA: Este agente tiene alta presión en la precondición — necesita que
  PatronesTacticos y AnalisisJueces ya tengan contenido (coherencia_sensor)

Agente 9: Recomendador
- System prompt: "Eres un estratega legal. Dado lo que predices que hará el
  abogado contrario, recomienda la contra-estrategia. Sé concreto: qué argumentar,
  qué jurisprudencia citar, qué preguntas hacer al testigo."
- Regiones que puede parchear: EstrategiaRecomendada
- Temperatura: 0.3
- NOTA: Necesita que PrediccionAudiencia ya tenga contenido
```

### Cómo funciona la autonomía (sin orquestador)

```
Tick 1:
  - Todas las regiones tienen presión máxima (vacías)
  - Los agentes 1,2,3 (Comprensión) ven presión alta en PerfilBasico/Historial
  - Los agentes 4,5,6 (Análisis) ven presión alta en PatronesTacticos
  - Los agentes 8,9 (Predictor/Recomendador) ven presión alta en sus regiones
    PERO las precondiciones no se cumplen (PatronesTacticos vacío → incoherencia alta)
  - Los agentes 1,2,3,4,5,6 trabajan en paralelo
  - Los agentes 8,9 esperan (la presión de incoherencia los bloquea naturalmente)

Tick 5:
  - PerfilBasico completado por Agente 1 → presión baja
  - Historial en progreso por Agente 3
  - Agente 4 (Buscador) encontró primer patrón (Art. 453)
  - Agente 5 (Calculador) toma el patrón y calcula: 39/50 = 78%
  - Agente 6 (Anomalías) detecta cambio temporal 2024

Tick 10:
  - PatronesTacticos tiene 3 patrones validados
  - AnalisisJueces tiene datos de Hernández
  - Ahora la presión de incoherencia baja en PrediccionAudiencia
  - Agente 8 (Predictor) se activa: tiene patrones + juez → genera predicción
  - Agente 9 (Recomendador) aún espera (necesita predicción)

Tick 13:
  - Predicción lista
  - Agente 9 se activa: genera estrategia basada en predicción + patrones
  - Si la estrategia no cita los patrones → sensor de coherencia da presión
  - Agente 9 parchea de nuevo incluyendo las citas

Tick 15-20:
  - Los agentes siguen refinando. Cada parche solo se acepta si reduce presión.
  - El decaimiento temporal hace que patrones no reforzados pierdan confianza.
  - Convergencia: stable_threshold = 3 ticks sin mejoras → TERMINADO.
```

---

## Paso 5: KernelConfig para el experimento legal

```rust
let config = KernelConfig {
    tick_interval_ms: 500,        // más lento que scheduling (LLMs son lentos)
    max_ticks: 50,                // máximo 50 iteraciones
    stable_threshold: 3,          // 3 ticks sin mejoras = done
    pressure_axes: pressure_axes, // los 4 ejes definidos arriba
    decay: DecayConfig {
        fitness_half_life_ms: 300_000,       // 5 minutos
        confidence_half_life_ms: 600_000,    // 10 minutos
        ema_alpha: 0.3,
    },
    activation: ActivationConfig {
        min_total_pressure: 0.5,   // threshold para activar propuestas
        inhibit_ms: 10_000,        // 10 seg cooldown post-patch
    },
    selection: SelectionConfig {
        min_expected_improvement: 0.1,
    },
};
```

---

## Paso 6: LLM Setup

### Opción A — Ollama (local, gratuito, más lento)

```bash
ollama pull qwen2.5:7b
OLLAMA_HOST=http://localhost:11434 cargo run -p legal-experiment
```

### Opción B — API compatible OpenAI (Claude via OpenRouter, más caro pero mejor)

```bash
# El vllm_client.rs ya soporta cualquier API OpenAI-compatible
OPENAI_API_BASE=https://openrouter.ai/api/v1 \
OPENAI_API_KEY=sk-or-... \
OPENAI_MODEL=anthropic/claude-sonnet-4-5 \
cargo run -p legal-experiment
```

---

## Paso 7: Criterio de éxito (OBLIGATORIO — no hay falsos positivos)

La radiografía generada DEBE contener, verificable por humano:

### Must-have (si falta alguno, FALLÓ):

- [ ] Nombre completo: "Fuentes Larraín, Andrés"
- [ ] Estudio: "Larraín & Cía"
- [ ] Volumen: ~50 causas
- [ ] Patrón Art. 453 N°2 CT detectado con frecuencia ~78%
- [ ] Tasa de éxito ante Hernández diferente a la de otros jueces
- [ ] Predicción concreta para O-892-2026: "Fuentes pedirá limitar prueba"
- [ ] Probabilidad asignada a la predicción
- [ ] Al menos 1 recomendación estratégica concreta

### Should-have (mejora la calidad):

- [ ] Patrón de conciliación post-rechazo detectado
- [ ] Rango de conciliación cuantificado (40-55%)
- [ ] Cambio temporal detectado (antes/después 2024)
- [ ] Artículos de ley más usados identificados
- [ ] Empresas representadas agrupadas por sector
- [ ] Contraargumento con jurisprudencia citada

### Métricas a medir:

```
1. Ticks hasta convergencia (target: < 25)
2. Total de parches aceptados vs rechazados
3. Presión total final (target: < 0.2)
4. Tokens consumidos total
5. Tiempo total de ejecución
6. ¿Los 9 agentes contribuyeron? (no debe haber agentes ociosos)
7. ¿Se respetó el orden natural? (Predictor DESPUÉS de Patrones)
```

---

## Paso 8: Validación contra API real del PJUD

DESPUÉS de que el MVP funcione con datos mock, validar que el formato es compatible con datos reales.

### API del PJUD (producción, funcional)

```
Base URL: https://lau-backend-760602975866.us-central1.run.app
```

### Test 1: Verificar que la API está viva

```bash
curl https://lau-backend-760602975866.us-central1.run.app/health
# Esperado: {"ok": true, "areas": ["civil","laboral","penal","cobranza","suprema","apelaciones"]}
```

### Test 2: Obtener tribunales laborales de Santiago

```bash
curl "https://lau-backend-760602975866.us-central1.run.app/api/live/catalogs/tribunals?codCompetencia=4&codCorte=30"
# Esperado: lista de tribunales laborales de la Corte de Santiago
# codCompetencia=4 es Laboral, codCorte=30 es C.A. de Santiago
```

### Test 3: Buscar causas laborales recientes por fecha

```bash
curl -X POST https://lau-backend-760602975866.us-central1.run.app/api/search/laboral/date \
  -H "Content-Type: application/json" \
  -d '{
    "fecDesde": "01/05/2026",
    "fecHasta": "15/05/2026",
    "corte": "30",
    "tribunal": "0"
  }'
# Esperado: {"count": N, "items": [...]}
# Cada item tiene: rol, fecha, caratulado, tribunal, detailToken
```

### Test 4: Obtener detalle de una causa (laboral funciona, civil tiene bug)

```bash
curl -X POST https://lau-backend-760602975866.us-central1.run.app/api/live/laboral/causa \
  -H "Content-Type: application/json" \
  -d '{"dtaCausa": "<detailToken de Test 3>"}'
# Esperado: JSON con litigantes, tramites, metadata
```

### Lo que debes verificar:

1. **Formato de datos mock vs real**: ¿los campos de tu mock coinciden con lo que devuelve la API?
2. **Nombres en MAYÚSCULAS**: el PJUD usa `"GONZÁLEZ MUÑOZ PEDRO"`, no `"González Muñoz, Pedro"`
3. **Formato de rol**: `O-{numero}-{año}` para laboral
4. **Fechas**: `DD/MM/YYYY` siempre, nunca ISO
5. **Si puedes**: sustituir algunos datos mock por datos reales de la API para la demo

### Notas sobre la API:

- **Laboral funciona** (necesita reCAPTCHA pero la API lo maneja internamente via `/api/search/laboral/date`)
- **Endpoints lentos**: 30-90 segundos por búsqueda (incluye resolución de captcha)
- **Timeout**: configurar HTTP client a >= 120 segundos
- **No necesita API key** — es pública
- **Búsqueda por nombre/RUT** requiere `_force_tspd_bypass: true` (protección WAF)

---

## Paso 9: Output esperado — lo que veremos en la demo de mañana

Cuando el experimento corra exitosamente, debe imprimir:

```
═══════════════════════════════════════════════════════════
  RADIOGRAFÍA: Fuentes Larraín, Andrés — Larraín & Cía
═══════════════════════════════════════════════════════════

PERFIL
  Abogado especialista en defensa de empresas en juicios laborales.
  50 causas analizadas (2019-2026). Sector predominante: energía y construcción.
  Tendencia: crecimiento sostenido. De 8 causas/año (2019) a 15 causas/año (2026).

CRONOLOGÍA
  2019-2022: Fase de crecimiento. Mayormente despidos injustificados.
  Conciliaba el 60% de las causas.
  2024-2026: Cambio de estrategia. Solo concilia el 35%.
  Mayor agresividad procesal desde enero 2024.

PATRONES TÁCTICOS
  [P1] Limitación de puntos de prueba — Art. 453 N°2 CT
       Frecuencia: 78% (39/50 causas) | Confianza: ALTA
       Evidencia: O-234-2023, O-567-2024, O-892-2026, ... (39 causas)

  [P2] Conciliación post-rechazo de limitación
       Frecuencia: 64% (9/14 ante Hernández) | Confianza: MEDIA
       Rango: 40-55% del monto demandado (promedio 47%)

  [P3] Cambio temporal de estrategia
       Pre-2024: 60% conciliación | Post-2024: 35% conciliación
       Confianza: ALTA (cambio estadísticamente significativo)

ANÁLISIS POR JUEZ
  Juez Hernández (23 causas):
    - Limitación rechazada: 60% (14/23)
    - Resultado para demandante: favorable 65%
  Otros jueces (16 causas con aud. prep.):
    - Limitación rechazada: 44% (7/16)
    - Resultado para demandante: favorable 50%

PREDICCIÓN PARA O-892-2026 (audiencia mañana)
  Juez: Hernández | Materia: Despido injustificado
  [92%] Fuentes pedirá limitar puntos de prueba (Art. 453 N°2 CT)
  [60%] Hernández rechazará la moción
  [64%] Si rechaza, Fuentes ofrecerá conciliación en audiencia siguiente
  [Rango] Oferta probable: $20M - $24.7M (47% de $45M demandados)

ESTRATEGIA RECOMENDADA
  1. Cuando pida limitar prueba: citar fallo O-1823-2025 del juez Carrera
     (rechazó la misma táctica, Hernández ha seguido esa línea)
  2. Defender puntos de prueba: (1) causal de despido, (2) monto remuneraciones
     (Fuentes siempre intenta sacar el punto 2)
  3. Si ofrece conciliar: no aceptar bajo 55% — el juez probablemente
     fallaría por más basado en historial

═══════════════════════════════════════════════════════════
  Convergencia: tick 18 | Parches: 34 aceptados / 12 rechazados
  Presión final: 0.08 | Tokens: ~45K | Tiempo: 4m 20s
  Agentes activos: 9/9
═══════════════════════════════════════════════════════════
```

---

## Resumen ejecutivo para la IA ejecutora

1. Clona `pressure-field-experiment`
2. Crea `crates/legal-experiment/` copiando la estructura de `schedule-experiment`
3. Implementa `LawyerProfileArtifact` (6 regiones) en vez de `ScheduleArtifact`
4. Implementa 4 sensores (Completitud, Evidencia, Coherencia, Profundidad) en vez de Gap/Overlap
5. Genera 50 causas mock con los 5 patrones embebidos (archivo `mock_data.rs`)
6. Configura 9 LLM actors (no 2-3 como en scheduling) con prompts legales
7. Corre el experimento y verifica los 8 criterios must-have
8. Valida formato contra la API real del PJUD
9. El output final debe ser la radiografía impresa en terminal

**No hay mediocridad aceptable.** Si el Buscador de Patrones no encuentra el patrón del Art. 453 con 78% de frecuencia, algo está mal en los datos o en el prompt. Si el Predictor no predice que Fuentes va a pedir limitar prueba mañana, el sensor de coherencia debe dar presión alta y forzar un re-patch. Si la Estrategia no cita jurisprudencia concreta, el sensor de evidencia debe rechazar el parche.

Los campos de presión garantizan que no se acepta trabajo mediocre — solo trabajo que REDUCE la presión. Eso es lo que lo hace distinto a un pipeline secuencial donde cada agente escribe lo suyo y nadie valida.
