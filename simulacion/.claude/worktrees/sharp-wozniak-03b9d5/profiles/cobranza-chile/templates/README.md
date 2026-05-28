# Templates de Mensajes — Perfil Cobranza-Chile

Estos templates son **específicos del negocio cobranza chileno**. Definen
cómo redactar emails/cartas/mensajes para deudores según mora, monto y
ley aplicable (21.394). El framework `mensajero` es agnóstico al negocio:
solo recibe `{subject, body, channel, to}` y los envía. La inteligencia
de "qué decir" vive acá.

## Convención

Cada template es un Markdown con frontmatter YAML:

```markdown
---
id: email_recordatorio
channel: email
tone: amistoso
required_data: [deudor_nombre, saldo, dias_mora, estudio_nombre, cobrador_nombre]
---
Asunto: Recordatorio de pago — {{deudor_nombre}}

Estimado/a {{deudor_nombre}},

Por medio de la presente le recordamos que tiene facturas pendientes por
un monto total de ${{saldo}}.

Días de mora: {{dias_mora}}.

Saludos cordiales,
{{cobrador_nombre}}
{{estudio_nombre}}
```

## Cómo se invocan

Hoy (MVP): el framework `mecánico` tiene los templates compilados en su
binario. Migración planeada: el framework `redactor` (futuro) recibirá
`--template profiles/cobranza-chile/templates/email_recordatorio.md` y
`--data <json>` y devolverá el draft listo. El nombre del template y
qué template usar para qué situación lo decide el flow del perfil, no
el framework.

## Reglas para nuevos templates

- **Nada de hard-coding**: si necesitás referencias a "cobranza" o "Ley
  21.394", debe ir en el template, no en código del framework.
- **Slots obligatorios** declarados en `required_data`. El framework valida
  antes de redactar.
- **Tone explícito**: `amistoso | formal | carta | urgente`. La UI puede
  ofrecer al usuario elegir.
- **Multi-canal**: si el mismo mensaje funciona en email y whatsapp,
  duplicar template con `channel:` distinto y body adaptado.
