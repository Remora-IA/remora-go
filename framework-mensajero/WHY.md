# WHY - Framework Mensajero

Mensajero existe porque enviar un mensaje por email, SMS o WhatsApp no
debería requerir que cada framework sepa cómo conectarse a cada canal.

Mensajero recibe un draft listo (destinatario, asunto, cuerpo, canal) y lo
envía. Es el cartero: no escribe la carta, solo la entrega.

## Problema Que Resuelve

Sin Mensajero, cada framework que necesita enviar un mensaje reimplementa la
conexión SMTP, la integración con Twilio o la API de WhatsApp. Mensajero
centraliza el envío.

## Relación Con Otros Frameworks

- **Hosting** provee las credenciales SMTP.
- **Sabio** provee el destinatario.
- **Mecánico** genera el draft del mensaje.
- **Tareas** registra el envío como evento.

Mensajero no genera contenido. Mecánico no envía. Sabio no envía.
