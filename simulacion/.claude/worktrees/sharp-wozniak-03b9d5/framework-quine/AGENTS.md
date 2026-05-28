# Framework Quine - Agentes

## Rol

Quine es el generador y revisor de frameworks. Su trabajo es crear nuevos frameworks y mantener la calidad de los existentes.

## Responsabilidades

1. **Crear frameworks** según especificaciones del usuario
2. **Detectar tipos** de frameworks existentes
3. **Revisar calidad** aplicando checklists apropiados
4. **Sugerir fixes** para frameworks incompletos
5. **Registrar frameworks** en el repositorio

## Cómo comunicarse con otros frameworks

Quine puede invocar otros frameworks para delegar trabajo:

### Invocar Alpha

```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-alfa
./frameworkalfa compile --echo-tree ../framework-echo/frameworkecho.json --out temp/spec.json
./frameworkalfa inspect --spec temp/spec.json
```

### Invocar Bravo

```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-bravo
./frameworkbravo compile --spec temp/spec.json --out temp/flow.json
```

### Invocar Charlie

```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-charlie
./frameworkcharlie generate --flow temp/flow.json --out temp/code/
```

## Tipos de frameworks y sus roles

| Tipo | Rol típico | Frameworks relacionados |
|------|-------------|-------------------------|
| inquisitivo | Descubrimiento, guía mediante preguntas | Echo → Alpha |
| nodos-arbol | Estructura jerárquica de conocimiento | Echo, Alfa |
| procesador | Transformación de datos | Charlie |
| integracion | Conexión con sistemas externos | - |
| automatizador | Ejecución de tareas repetitivas | - |

## Loop con otros frameworks

```
Usuario → Quine → Crear/Revisar framework → Registrar → Feedback
              ↓
          Echo (descubrir)
              ↓
          Alpha (compilar spec)
              ↓
          Bravo (generar flujo)
              ↓
          Charlie (codificar)
```

## Comandos de integración

- `quine review --path <fw>` - Revisar framework y aplicar checklists
- `quine register --path <fw> --type <tipo>` - Registrar en repositorio
- `quine types` - Ver tipos disponibles y sus checklists

## Checklists por tipo

### Inquisitivo
- preguntas-guia, log-qa, signal-fatiga, semáforo-decisión
- no-ofrecer-temprano, una-pregunta-a-la-vez

### Nodos-Arbol
- estructura-nodos, jerarquia-capas, estados-nodo
- add-nodo-cmd, validate-nodo-cmd, show-tree-cmd

### Base (todos)
- INITIAL_PROMPT.md exists, rol, filosofia, comandos
- AGENTS.md exists
- README.md exists
- cmd/main.go exists
- internal/paladin exists
- go.mod exists

## Notas

- Quine detecta tipos automáticamente basándose en la estructura
- Los checklists se aplican según el tipo detectado
- Un framework pasa el estándar cuando no tiene items [required] fallidos
- El registro se guarda en frameworks.json dentro de Quine