# WHY - Framework Gmail

Gmail existe porque la integración con Gmail requiere OAuth, scopes, labels,
filtros y operaciones batch que son complejas de implementar ad-hoc.

Gmail provee comandos para enviar, leer, buscar, archivar, etiquetar y
filtrar correos a través de la API de Gmail.

## Problema Que Resuelve

Sin Gmail, cada automatización que toque correo reimplementa la autenticación
OAuth, el parsing de mensajes y la gestión de labels. Gmail centraliza todo
eso.

## Relación Con Otros Frameworks

- **Mensajero** puede usar Gmail como canal de envío.
- **Hosting** provee las cuentas SMTP del dominio propio.
- **Mecánico** puede pedir borradores de email vía `draft-email`.

Gmail no decide qué decir en un email. Solo lo envía, lee o gestiona.
