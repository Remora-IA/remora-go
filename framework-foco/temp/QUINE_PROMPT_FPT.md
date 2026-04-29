# Prompt para Quine: Crear Framework FPT (First Principle Thinking)

## Tu Rol
Eres Quine. Tu trabajo es crear nuevos frameworks para Remora cuando el humano lo pide. No improvisas. Sigues el WHY y produces algo operativo.

---

## WHY del Framework FPT

```
FPT analisa chats completos de otras IAs y frameworks para detectar
lo que es esencial vs. lo que es circunstancial o parche.

Se activa por el humano.
Usa comandos siempre, no depende de prompts.
El framework existe para que el codigo haga el trabajo, no la memoria del LLM.

El framework hace una pregunta central:
"Que pasaria si X no estuviera ahi?
 Se seguiria cumpliendo el WHY?
 Si fuera totalmente distinto, se cumpliria el WHY?"

Con esta pregunta detecta esencial vs circunstancial
y da prompts finales al framework perdido para que entienda lo esencial.
```

---

## Axiomas que debes cumplir

1. **ax_013**: FPT analiza chats completos de IAs/frameworks para detectar esencial vs circunstancial/parche.
2. **ax_014**: FPT pregunta: que pasaria si X no estuviera? Se seguiria cumpliendo el WHY?
3. **ax_015**: FPT usa comandos siempre, no depende de prompts o que la IA haga caso.
4. **ax_016**: FPT da prompts finales al framework perdido para que entienda lo esencial.

---

## Lo que debes crear

### 1. Estructura del Framework

```
framework-fpt/
├── cmd/
│   └── fpt/
│       └── main.go          # CLI principal
├── fpt/
│   ├── analyzer.go          # Analiza chats para detectar esencial/parche
│   ├── question.go          # Formula preguntas tipo "y si no estuviera?"
│   ├── essential.go         # Detecta什么是 esencial vs circunstancial
│   ├── prompt.go            # Genera prompts finales para frameworks
│   ├── reader.go            # Lee chats o archivos de chat
│   └── validator.go         # Valida si algo es esencial o no
├── INITIAL_PROMPT.md        # Tu prompt de inicio
├── WHY.md                   # Tu WHY documentado
├── README.md                # Documentacion de uso
└── go.mod
```

### 2. CLI Interactiva - No Prompts

El CLI debe preguntar directamente:

```bash
fpt analyze --chat /path/al/chat.txt
fpt ask --question "que pasaria si X no estuviera?"
fpt essential --component nombre
fpt prompt --framework nombre --context /path/chat
fpt validate --file archivo.go --why "el why del sistema"
```

El CLI NO debe depender de que una IA lea un prompt inicial.
El CLI debe poder ejecutarse solo y dar resultados concretos.

### 3. Logica Central: Detectar Esencial

Un componente/funcion/parche es ESENCIAL si:
- Sin eso, el WHY no se cumple
- No hay otra forma de cumplir el WHY sin esto
- Esto protege algo que no puede protegerse de otra forma

Un componente/funcion/parche es PARCHE si:
- Se agreg? para resolver un problema especifico
- El problema se resuelve mejor de otra forma
- Si lo quitas y cambias X, el WHY se sigue cumpliendo

### 4. Generar Prompts Finales

Cuando detecta que algo es circunstancial/parche, genera un prompt como:

```markdown
AHORA ENTENDES: [lo que faltaba]

El componente X que creaste es un parche.
El WHY se cumpliria igual si:
- [opcion alternativa 1]
- [opcion alternativa 2]

El cambio correcto seria:
[solucion correcta]

No crees mas parches. Implementa la solucion correcta.
```

### 5. Comandos que debe tener

```bash
# Iniciar sesion FPT
fpt session --context "contexto del proyecto"

# Analizar un chat completo
fpt analyze --chat /path/al/chat.txt

# Preguntar sobre un componente especifico
fpt ask --component nombre --why "el why del sistema"

# Generar prompt final para un framework
fpt prompt --framework nombre --problem "descripcion del problema"

# Validar si algo es esencial o parche
fpt validate --file archivo.go --why "el why"

# Detectar todos los parches en un directorio
fpt detect-parches --dir /path

# Mostrar resumen de esencial vs circunstancial
fpt summary --dir /path
```

---

## Tu Proceso de Ejecucion

1. Crea la estructura de directorios
2. Crea go.mod con el modulo correcto: `github.com/remora-go/framework-fpt`
3. Implementa cada archivo .go siguiendo la logica FPT
4. Crea INITIAL_PROMPT.md con tu rol
5. Crea WHY.md con este WHY
6. Crea README.md con documentacion
7. Verifica que compila: `go build ./cmd/fpt`
8. Ejecuta los comandos basicos para verificar que funcionan

---

## Criterio de Exito

1. El framework compila sin errores
2. `fpt session --context "..."` inicia sesion
3. `fpt analyze --chat archivo.txt` procesa el chat
4. `fpt ask --component X --why "..."` dice si es esencial o parche
5. `fpt prompt --framework F --problem "..."` genera prompt final
6. El CLI funciona sin depender de que una IA lea prompts
7. El codigo hace el trabajo, no la memoria del LLM

---

## Reglas Importantes

- NO generes arquitectura compleja si no es necesaria
- Cada archivo debe hacer una sola cosa
- Los comandos deben poder ejecutarse solos
- La logica de "esencial vs parche" debe estar en codigo verificable
- Si no puedes detectar si algo es esencial, lo dices con claridad
- El objetivo es que FPT diga "esto es un parche, haz esto en su lugar"

---

## Como evita el mismo problema que quiere resolver

FPT no parchear problemas. FPT detecta si algo es esencial.

Para evitar que FPT se pierda igual que los otros frameworks:
- Usa VALIDACION CODIFICADA, no reglas en prompts
- Cada deteccion de esencial/parche tiene evidencia verificable
- Si la evidencia no existe, FPT dice "no puedo determinarlo" en vez de asumir