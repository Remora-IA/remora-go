# Axiomas Generales — Remora

## Axioma rector

**Un flujo está listo cuando cualquier framework puede entrar, actuar, pedir ayuda, esperar o salir del equipo usando solo capabilities declaradas, contratos verificables y trazas auditables, sin cables por nombre ni conocimiento oculto.**

Este axioma resume la dirección del sistema: los flujos son definidos de antemano, pero la actuación dentro del flujo es autónoma y acotada por contrato.

## Axiomas del sistema

### 1. Autonomía acotada

Cada framework puede decidir si debe actuar, esperar, pedir ayuda, preguntar al humano o terminar, pero solo dentro de las capabilities, comandos, inputs, outputs y políticas que declara su manifest.

### 2. Composición por capabilities

El orquestador, el frontend y las reglas de flujo no deben decidir por nombre de framework. Deben pedir una capability y dejar que el registry resuelva qué framework la provee.

### 3. Flujos como equipos declarativos

Un flujo define participantes, objetivo, capabilities permitidas, dependencias y límites. No prescribe cada turno como un guion rígido.

### 4. Contratos antes que prompts

La integración entre frameworks se define por JSON, manifests, commands, schemas, requires, produces y policies. Los prompts pueden guiar estilo y razonamiento, pero no reemplazan contratos.

### 5. Proceso auditable

No basta con que la respuesta parezca correcta. Cada turno debe poder explicar qué framework actuó, qué capability reclamó, qué fuente usó, qué produjo, qué políticas cumplió y si pidió ayuda o fallback.

### 6. Sin fallbacks silenciosos

Si un framework cambia de fuente, engine, modelo o estrategia, debe estar declarado por policy y quedar registrado en trace. Si no puede responder con evidencia suficiente, debe fallar o pedir ayuda.

### 7. Estado con dueño claro

Cada dato persistente tiene un dueño canónico por capability. Los JSONs de sesión son estado interno, artifacts o trazas; no deben convertirse en fuentes de verdad ocultas.

### 8. Overlays no controlan ejecución

Los overlays pueden ajustar tono, formato y contexto de presentación. No pueden cambiar routing, source of truth, fallback, permisos, mutaciones ni decisión de capability.

### 9. Paladin estandariza el futuro

Cada nueva regla arquitectónica debe volverse validable por Paladin cuando sea posible. Si algo depende de recordar manualmente una convención, todavía no está suficientemente estandarizado.

### 10. Extender debe ser más barato que cablear

Agregar, quitar o reemplazar un framework debe requerir declarar capabilities y pasar lint/tests, no modificar rutas especiales en el orquestador ni reglas name-based.

## Criterio de listo

El sistema se acerca a listo cuando una IA nueva puede leer `ARCHITECTURE.md`, este archivo, los manifests y un trace de Paladin, y entender:

1. qué equipo participa en el flujo,
2. qué capabilities existen,
3. quién puede proveer cada capability,
4. por qué actuó cada framework,
5. qué evidencia usó,
6. qué falta para completar la tarea,
7. y dónde validar si se rompió el contrato.

## Axiomas especializados

- `framework-sabio/AXIOMS.md`: contrato de Sabio como experto de datos con SQLite como fuente primaria, capabilities explícitas, sin fallback silencioso y overlays limitados a presentación.
