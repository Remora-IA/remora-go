# Agentes - Framework Foco

## Rol

Foco es el priorizador y orquestador del día. Coordina el trabajo entre
frameworks para mantener el foco en el WHY.

## Comunicación con otros frameworks

### Invocar Echo (descubrimiento)
```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-echo
./frameworkecho ask
./frameworkecho answer --text "..."
```

### Invocar Alfa (compilar spec)
```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-alfa
./frameworkalfa compile --echo-tree ../framework-echo/frameworkecho.json --out temp/spec.json
```

### Invocar Bravo (generar flujo)
```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-bravo
./frameworkbravo compile --spec temp/spec.json --out temp/flow.json
```

### Invocar Charlie (codificar)
```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-charlie
./frameworkcharlie generate --flow temp/flow.json --out temp/code/
```

## Flujo de trabajo

```
Usuario → Foco (prioriza, mantiene WHY)
            ↓
        Echo (descubre requisitos si hay ambigüedad)
            ↓
        Alfa (compila spec)
            ↓
        Bravo (genera flujo)
            ↓
        Charlie (codifica)
```

## Cuándo invocar otros frameworks

| Situación | Framework | Comando |
|-----------|-----------|---------|
| Ambigüedad sobre requisitos | Echo | `echo ask` / `echo answer` |
| Spec incompleta | Alfa | `alfa compile` |
| Flujo unclear | Bravo | `bravo compile` |
| Código no avanza | Charlie | `charlie generate` |

## Notas

- Foco es el "director de orquesta" del día
- No invocas frameworks por capricho, solo cuando hay pre-conflictos lógicos
- Siempre mantén el WHY visible
- Ofrece opciones antes de invocar otro framework
