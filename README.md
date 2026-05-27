# Remora

Librería que permite a **founders no-técnicos (1–3 personas)** usar IAs como Claude para construir productos donde **agentes trabajan en equipo**, sin reinventar la rueda en cada proyecto.

Dos garantías:

1. El enjambre se adapta al problema.
2. El enjambre trabaja correctamente en equipo.

Primer caso real: **Kobra-Carolina** — agente conversacional de cobranza por WhatsApp para Somos Rentable (Chile). Ver [examples/kobra-carolina](examples/kobra-carolina).

## Cómo se usa Remora (workflow del founder)

Remora tiene **dos capas distintas** que el founder usa en momentos distintos:

### Capa 1 — Diseño (antes de codear, con el cliente)

| Framework | Para qué sirve | Cuándo lo usas |
|---|---|---|
| [echo](framework-echo) | Conversar con tu cliente real para entender el dolor y oportunidad de automatización | Antes de escribir código. Resultado: árbol Echo con AXIOM/THEORY/TASK/PAIN/OPPORTUNITY validado. |
| [alfa](framework-alfa) | Compilar el árbol Echo a una spec ejecutable (reglas de negocio, flujo ideal, variables críticas) | Después de Echo, antes de implementar. Resultado: `alfa_spec.json` y `ideal_flow.json`. |

Estas dos no se ejecutan dentro del producto. Se usan **al diseñar**.

### Capa 2 — Runtime (lo que ejecuta tu producto)

| Framework | Para qué sirve | Cuándo lo usas |
|---|---|---|
| [llm](framework-llm) | Llamar a Claude (o el modelo que elijas) sin escribir wrappers HTTP | Siempre que tu agente piense. |
| [agent](framework-agent) | Primitiva de agente conversacional con estado, traza, turn loop | Cada agente de tu producto. |
| [channels](framework-channels) | Transporte (WhatsApp, consola, email, SMS) abstraído del agente | Cómo tu agente habla con el mundo. |
| [store](framework-store) | Persistencia de conversaciones multi-tenant | Cuando un agente atiende a más de un usuario. |
| [paladin](framework-paladin) | Trazas semánticas colectivas para observabilidad | Siempre. Sale gratis con el agente. |
| [swarm](framework-swarm) | Coordinación bio-inspirada entre múltiples agentes (estigmergia) | Cuando necesitas varios agentes trabajando en paralelo sin pisarse. |

### Capa bisagra — Verificación

| Framework | Para qué sirve | Cuándo lo usas |
|---|---|---|
| [bravo](framework-bravo) | Comparar lo que tu producto hizo (trace real) vs lo que esperabas (ideal_flow de Alfa) | En diseño (definir el ideal_flow) y en runtime (verificar que el código lo cumplió). |

## Sinergia

**Diseño** (Echo → Alfa) te asegura que estás construyendo lo correcto.
**Runtime** (agent + channels + store + llm, con paladin observando) ejecuta lo diseñado.
**Bravo** cierra el loop comparando expectativa vs realidad.

Si saltas el diseño, terminas construyendo un producto para un problema inventado. Si saltas el runtime, tienes un manifiesto sin código. Si saltas Bravo, no sabes si el código resolvió el dolor real o solo corrió sin errores.

## Capacidades de apoyo

Frameworks que NO son del trípode pero que aportan capacidades específicas cuando las necesitas:

- [charlie](framework-charlie), [excel](framework-excel), [foco](framework-foco), [gmail](framework-gmail), [quine](framework-quine), [flujo](remora-flujo) — capacidades especializadas. Úsalas cuando tu agente necesite una de esas habilidades concretas.

## Estado actual y deuda honesta

- **Runtime funcional end-to-end**: validado con Kobra-Carolina. Puedes construir un agente conversacional persistente con consola hoy mismo.
- **Capa de diseño parcialmente conectada**: Echo y Alfa existen como CLIs pero su output todavía requiere trabajo manual para llegar al system prompt de un agente. La integración natural sería un comando `alfa export-system-prompt --spec alfa_spec.json` que aún no existe.
- **Bravo runtime de equipo**: Bravo verifica una ejecución; falta la capa que verifica que el output **colectivo** de un enjambre cumple la intención. Issue abierto.
- **WhatsApp integration**: stub funcional. Para producción se necesita decidir Twilio vs Meta Cloud API + templates aprobados.
- **Module paths inconsistentes**: ver [issue de seguimiento](https://github.com/Remora-IA/remora-go/issues). Hoy todos los frameworks declaran `github.com/remora-go/...` pero el repo es `github.com/Remora-IA/remora-go`. Hasta arreglarse, los proyectos externos usan `replace` directives.

## Instalación (provisional)

```bash
git clone https://github.com/Remora-IA/remora-go.git
```

En tu `go.mod`:

```go
require (
    github.com/remora-go/framework-agent    v0.0.0
    github.com/remora-go/framework-channels v0.0.0
    github.com/remora-go/framework-llm      v0.0.0
    github.com/remora-go/framework-paladin  v0.0.0
    github.com/remora-go/framework-store    v0.0.0
)

replace (
    github.com/remora-go/framework-agent    => ../remora-go/framework-agent
    github.com/remora-go/framework-channels => ../remora-go/framework-channels
    github.com/remora-go/framework-llm      => ../remora-go/framework-llm
    github.com/remora-go/framework-paladin  => ../remora-go/framework-paladin
    github.com/remora-go/framework-store    => ../remora-go/framework-store
)
```

## Filosofía

Ver [nuevo_mapa.md](nuevo_mapa.md) y los READMEs individuales para el racional detrás de cada pieza.

## Para founders que llegan nuevos

Empieza por leer [examples/kobra-carolina/README.md](examples/kobra-carolina/README.md). Está armado como referencia para que puedas construir tu propio Carolina-equivalente.
