# WHY - Framework Mecánico

Mecánico existe porque detectar un problema no es lo mismo que corregirlo.

Auditor encuentra anomalías. Mecánico propone cómo arreglarlas, muestra el
plan al usuario y aplica solo cuando hay confirmación explícita.

## Problema Que Resuelve

Sin Mecánico, cada fix es manual: abrir el JSON, buscar el registro, editar
el campo, rezar que no se rompa otra cosa. Mecánico genera propuestas
trazables con audit-trail.

## Relación Con Otros Frameworks

- **Auditor** provee los findings que Mecánico consume.
- **Mensajero** puede enviar notificaciones después de aplicar fixes.
- **Tareas** registra el fix como evento completado.

Mecánico no detecta. Auditor no corrige. Mecánico no envía mensajes.
