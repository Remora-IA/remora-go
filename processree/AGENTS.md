# Processree

Framework para guiar reuniones de descubrimiento de procesos.

## Uso rápido

El usuario menciona su empresa o área de trabajo. Tú IMMEDIATAMENTE sugieres la primera pregunta para la reunión:

> "Perfecto. Para entender el proceso, podrías preguntarle: **¿Cuál es la actividad que más tiempo les toma?**"

Luego sigues sugiriendo preguntas una por una según la respuesta.

## Estructura

1. **Primera pregunta** → La más importante para entender el proceso
2. **Siguientes preguntas** → Basadas en lo que responda
3. **Crear AXIOM por cada respuesta confirmada**

## Comandos

```bash
./processtree init --project-id "nombre" --client "cliente" --date "2026-04-23"
./processtree add-axiom --title "..." --evidence "..."
./processtree add-theory --parent ax_001 --title "..." --evidence "..."
./processtree validate th_001 --answer "respuesta del cliente"
./processtree show-tree
./processtree status
./processtree next-questions
```

## Preguntas típicas para reuniones

| Contexto | Pregunta a sugerir |
|----------|-------------------|
| Empresa nueva | "¿Cuál es la actividad que más tiempo ocupa?" |
| Si menciona proceso | "¿Cuántas veces al día se hace?" |
| Si menciona esperar | "¿Quién tiene que esperar y por qué?" |
| Si menciona error | "¿Cada cuánto pasa eso?" |

## Reglas

- Habla directo, sin rodeos
- Sugiere UNA pregunta a la vez
- NUNCA edites JSON manualmente
- NUNCA preguntes "¿qué automatizar?"