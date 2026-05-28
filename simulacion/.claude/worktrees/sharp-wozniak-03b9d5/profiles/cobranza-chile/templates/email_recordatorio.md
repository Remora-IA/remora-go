---
id: email_recordatorio
channel: email
tone: amistoso
required_data: [deudor_nombre, saldo, dias_mora, estudio_nombre, cobrador_nombre]
---
Asunto: Recordatorio de pago - {{deudor_nombre}} - {{estudio_nombre}}

Estimado/a {{deudor_nombre}},

Por medio de la presente le recordamos que tiene facturas pendientes por un monto total de ${{saldo}}.

Detalle:
- Días de mora: {{dias_mora}} días

Le solicitamos gentilmente regularizar esta situación a la brevedad.

Saludos cordiales,
{{cobrador_nombre}}
{{estudio_nombre}}
