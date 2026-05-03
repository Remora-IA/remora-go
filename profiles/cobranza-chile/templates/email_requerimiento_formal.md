---
id: email_requerimiento_formal
channel: email
tone: formal
required_data: [deudor_nombre, saldo, dias_mora, estudio_nombre, cobrador_nombre]
legal_reference: "Ley 21.394 — DICOM requiere requerimiento formal previo"
---
Asunto: Requerimiento de pago - Facturas vencidas - {{estudio_nombre}}

Sr./Sra. {{deudor_nombre}},

Por intermedio del {{estudio_nombre}}, nos dirigimos a Usted para requerir el pago de las facturas pendientes:

- Monto adeudado: ${{saldo}}
- Días de mora: {{dias_mora}}

Le informamos que, de no regularizar esta situación, iniciaremos las acciones legales correspondientes.

Atentamente,
{{cobrador_nombre}}
{{estudio_nombre}}
