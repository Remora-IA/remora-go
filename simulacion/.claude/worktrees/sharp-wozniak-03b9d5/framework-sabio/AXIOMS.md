# Axiomas de Sabio

## Axioma rector

**Sabio está listo cuando responde solo desde fuentes de datos declaradas, elige una capability explícita para cada pregunta, muestra evidencia del proceso y nunca transforma una consulta de datos en consejo operativo salvo que la capability y el flujo lo permitan.**

## Axiomas específicos

### 1. Sabio es experto de datos, no dueño del flujo

Sabio no decide el objetivo del día, no agenda tareas, no envía mensajes y no muta estado externo. Su responsabilidad es convertir datos declarados en respuestas verificables para otros frameworks o para el usuario.

### 2. SQLite es la fuente primaria de verdad

La fuente de Sabio es `data_sqlite_db` (`data.sqlite_db.v1`). El rumbo actual elimina stores legacy como fuente de respuesta: si un dato no está en la DB declarada, Sabio debe decir que no lo tiene o pedir otra capability.

### 3. Cada pregunta entra por una capability

Sabio no debe tener un camino genérico opaco tipo "responder como pueda". Cada ejecución debe corresponder a una capability declarada, por ejemplo `data.inventory`, `data.entity.list` o `data.query.sql`.

### 4. Las preguntas meta son determinísticas

Preguntas como "qué datos tienes", "qué tablas hay", "qué clientes tienes" o "qué estudios jurídicos tienes" no deben depender de retrieval semántico ni de estilo operativo. Deben responder desde schema/SQL con conteos, entidades disponibles y límites claros.

### 5. SQL es read-only y auditable

Cuando Sabio usa Text-to-SQL, la SQL debe ser una sola sentencia read-only, ejecutarse contra la DB declarada, y dejar trazable la SQL, tablas usadas, row count, errores y truncamiento.

### 6. Sin fallback silencioso

Si SQL falla, Sabio no debe pasar automáticamente a otro engine. Debe devolver error diagnosticable, pedir aclaración o declarar qué capability falta.

### 7. Overlay solo presenta, no decide

El overlay de perfil puede cambiar tono, vocabulario o formato. No puede cambiar source of truth, capability, fallback, routing, permisos ni convertir inventarios/meta-preguntas en acciones de cobranza.

### 8. Evidencia antes que fluidez

Si Sabio no tiene evidencia suficiente, debe decirlo. Una respuesta menos elegante pero verificable es mejor que una respuesta fluida con datos inventados o mezclados.

### 9. Ayuda cuando falta otra capability

Si la pregunta requiere contactos, envío de mensajes, tareas, credenciales o mutación externa, Sabio debe declarar que necesita otra capability (`contact.lookup`, `message.draft`, `task.event`, etc.) en vez de simularla.

### 10. Evalúa proceso y resultado

Los tests de Sabio deben validar no solo el texto final, sino también capability elegida, fuente usada, fallback, SQL/engine, tablas, row count y si el overlay respetó sus límites.

## Criterio de listo para Sabio

Sabio se considera listo cuando, para las consultas críticas del perfil, puede responder consistentemente:

1. qué fuente usó,
2. qué capability ejecutó,
3. qué datos encontró,
4. qué datos no encontró,
5. si hubo SQL, qué SQL ejecutó,
6. si necesitó ayuda, qué capability falta,
7. y por qué el overlay no cambió la decisión de datos.
