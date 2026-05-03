---
id: carta_requerimiento
channel: email
tone: carta
required_data: [deudor_nombre, saldo, dias_mora, estudio_nombre, cobrador_nombre, fecha]
legal_reference: "Art. 37 Ley 21.394"
---
Asunto: Carta de requerimiento formal - Art. 37 Ley 21.394

{{estudio_nombre}}, {{fecha}}

Sr./Sra. {{deudor_nombre}}

REFERENCIA: Requerimiento de pago

Por intermedio de este estudio jurídico, nos dirigimos a Usted para notificarle la existencia de la siguiente deuda:

- Monto total: ${{saldo}}
- Días de mora: {{dias_mora}}

Se le otorga un plazo de 10 días hábiles para el pago total de la deuda.

Atentamente,
{{cobrador_nombre}}
{{estudio_nombre}}
