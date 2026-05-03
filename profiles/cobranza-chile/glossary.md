# Glosario de Cobranza - Chile

## Actores
- **Deudor**: Persona natural o jurídica que debe dinero. NO decir "cliente" (el cliente es el estudio/abogado que nos contrató).
- **Acreedor**: El cliente del estudio jurídico que nos contrató para cobrar (ej: bancos, empresas).
- **Cobrador**: Trabajador del estudio que realiza las gestiones de cobranza.
- **Codeudor**: Persona que respalda la deuda del deudor principal.

## Documentos y Estados
- **Factura / Documento**: Título de crédito o factura impaga. En la base: billing_documents.
- **Mora**: Estado de retraso en el pago. Se mide en días.
- **Días vencidos**: Cantidad de días desde que venció la factura hasta hoy.
- **Saldo**: Monto pendiente de pago.
- **Residuo (residue)**: Monto que queda pendiente después de pagos parciales.

## Etapas del Proceso (Ley 21.394)
1. **Cobranza extrajudicial**: Gestiones previas a demanda (llamadas, mails, cartas).
2. **Carta de requerimiento de pago**: Notificación formal que debe enviarse antes de DICOM (Art. 37).
3. **DICOM**: Informe de deuda al sistema de información comercial. Solo después de 60 días y carta certificada.
4. **Prejudicial**: Acciones previas a la demanda judicial.
5. **Judicial**: Demanda ejecutiva o ordinaria.

## Acciones de Cobranza
- **Gestión**: Cualquier contacto con el deudor (llamada, email, visita).
- **Recordatorio**: Primer contacto amistoso.
- **Requerimiento formal**: Carta con tono legal.
- **Negociación**: Acuerdo de pago, condonación parcial, refinanciamiento.
- **Escalamiento**: Pasar a siguiente etapa (ej: de extrajudicial a DICOM).

## Métricas Clave
- **Probabilidad de recupero**: Chance de cobrar basada en histórico del deudor.
- **Monto recuperable**: Saldo × probabilidad de recupero.
- **Costo de oportunidad**: Lo que dejamos de ganar si no cobramos.
- **Score de prioridad**: Fórmula interna para ordenar qué deudores atacar primero.

## Unidades Monetarias
- **CLP**: Peso chileno (default si no se especifica).
- **UF**: Unidad de Fomento. Solo usar si explícitamente se menciona.
- **USD**: Dólar. Solo si la factura está en dólares.

## Tono y Comunicación
- Primero siempre educado y profesional, escalando a formal/legal solo si es necesario.
- Nunca agresivo ni amenazante (ilegal por Ley 21.394 Art. 38).
- Claro sobre consecuencias legales reales, no exageradas.
