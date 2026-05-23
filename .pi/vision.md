# Visión del producto — lau-ai
**Actualizado:** 2026-05-23

## Hipótesis central
"Creemos que el abogado litigante independiente o de estudio pequeño que gestiona 20 o más causas activas tiene el problema de que Railcase y la OJV cubren el monitoreo básico pero requieren carga manual constante para mantenerse actualizados, dejando sin cobertura los tribunales fuera de OJV (policía local, tributarios, ambientales), y está dispuesto a pagar una suscripción mensual por una plataforma que sincronice automáticamente su cartera completa sin intervención manual porque el costo de un plazo vencido —aunque infrecuente— es reputacionalmente inaceptable. Sabremos que esto es verdad cuando el tiempo de preparación del listado diario de tareas pase de 30–40 minutos a menos de 5 minutos para al menos 10 abogados litigantes activos de pago, en los primeros 60 días tras el lanzamiento."

## Usuario objetivo
Abogado litigante con cartera de 20 a 60 causas activas simultáneas, en estudio pequeño (2–10 personas) o ejercicio independiente, con sede fuera de Santiago o en regiones (Quinta Región como proxy validado). Usa la OJV todos los días como primera fuente y Railcase como doble chequeo. Tiene Railcase desactualizado porque la carga manual de causas requiere tiempo que no tiene. Al menos una parte de su cartera involucra tribunales que no están en la OJV. No es el socio senior —es el abogado mid-level o el independiente que lleva todo solo, sin equipo de soporte que haga el seguimiento por él. Lo que lo diferencia de "los abogados" en general: litiga, gestiona volumen, vive con el miedo al plazo vencido, y ya paga por redundancia.

## Bets activos (iteración actual)
| Bet | Hipótesis que valida | Métrica de éxito | Deadline |
|-----|---------------------|-----------------|---------|
| Bet A — Onboarding automático para abogados individuales | Si eliminamos la carga manual de causas (el punto de fricción que mantiene Railcase desactualizado), los abogados litigantes con 20+ causas cambian de herramienta sin necesidad de un proceso comercial largo | 5 abogados litigantes activos de pago en los primeros 30 días tras lanzamiento del onboarding automático; tasa de activación (causas sincronizadas en las primeras 48h) > 80% | Sprint 4 — julio 2026 |
| Bet B — API enterprise con Auth0 para departamentos legales corporativos | Si ofrecemos el único acceso programático documentado al PJUD con autenticación OAuth2 y SLA explícito, los departamentos legales corporativos y proveedores de ERP/CRM pagan por integración en lugar de construir scraping propio | 1 contrato enterprise firmado con integración activa al cabo de 90 días de comercialización de la API | Septiembre 2026 |

*Nota: Bet A y Bet B son rutas alternativas, no simultáneas. El equipo no tiene capacidad de ejecutar ambas en paralelo. La decisión de qué bet priorizar requiere una segunda entrevista con al menos dos abogados litigantes adicionales antes de comprometer ingeniería en cualquiera de las dos rutas.*

## Ventaja competitiva real
La combinación de dos cosas que ningún competidor tiene a la vez: (1) API con autenticación Auth0/OAuth2, SLA documentado y cobertura de todas las jurisdicciones del PJUD en tiempo real —Boostr.cl tiene API pero sin SLA, sin Auth0 y en modelo pay-per-use sin garantías; CaseTracking tiene integración pero no la expone como API consumible—; y (2) cobertura de tribunales fuera de la OJV (policía local, tributarios, ambientales) que todos los competidores basados en OJV ignoran estructuralmente. Replicar esto requiere mantener integración técnica funcional con sistemas del PJUD que no tienen API oficial pública, más la infraestructura Auth0 y el SLA operativo. CaseTracking podría intentarlo en 12–18 meses pero su modelo de negocio actual es SaaS de interfaz, no plataforma de datos —el incentivo a canibalizarse es bajo.

## Fuera de scope (esta iteración)
- Automatización de cálculo de plazos procesales intermedios (no están explícitos en las resoluciones; requiere interpretación legal ambiciosa que el experto marcó como "casi imposible de abarcar")
- Expansión a LATAM o cobertura de tribunales fuera de Chile
- Perfilado argumentativo de contraparte y jueces (puede ser diferenciador en juicios complejos de alto monto, pero requiere validación técnica de viabilidad con datos públicos de OJV antes de comprometer desarrollo)
- Módulo de gestión de honorarios y cobros (relCase y Pocket Lawyer lo tienen; no es el dolor prioritario del segmento objetivo)
- Integración con tribunales que tramitan en físico (el experto señaló que algunos son "casi imposibles" de enlazar informáticamente)
- Estudios jurídicos grandes en Santiago (proceso de compra formal y largo; no es el early adopter)

## Supuestos críticos sin validar
- El onboarding automático basado en el perfil de la OJV es técnicamente posible sin intervención manual del abogado (es la hipótesis central del Bet A — no ha sido evaluada por el equipo técnico)
- Los abogados con tribunales fuera de OJV representan un porcentaje suficientemente alto de la cartera del usuario objetivo para ser un diferenciador de compra (Juan mencionó cinco tipos de tribunales no cubiertos, pero no cuantificó qué fracción de su trabajo cae en ellos)
- Existe disposición a pagar una suscripción mensual con precio concreto — la afirmación "estaba dispuesto a pagar millones" del placeholder anterior no fue validada con cifra ni con Juan Magasich en la entrevista del 23 de mayo
- El abogado junior o mid-level puede actuar como champion interno y convencer a socios mayores en semanas — inferido del proceso de adopción de Magnar en el estudio de Juan, no verificado en otros estudios
- La confidencialidad de causas (familia, penal) no bloquea el onboarding —el experto señaló que "si es un abogado escrupuloso, está más atado de manos"; impacto real en tasa de activación sin medir
- El perfilado argumentativo de contraparte es técnicamente viable con los datos públicos de la OJV —Juan expresó explícitamente que no lo sabe

---
<!-- HISTORIAL
2026-05-23: Vision inicial basada en evidencia de entrevista Juan Magasich (insight-20260523-090000.md) + diagnóstico de alineación (pm-brief-20260523-090000.md) + análisis de mercado (mercado-20260522-220123.md) + análisis competitivo (competencia-20260522-221217.md). Pendiente validación directa con fundadores (Bastián, Tomás, JP) y segunda entrevista con abogado litigante para validar Bet A vs Bet B.
-->
