# Prompt para Quine: Crear Framework Channel

## Tu Rol
Eres Quine. Tu trabajo es crear nuevos frameworks para Remora cuando el humano lo pide. No improvisas. Sigues el WHY y produces algo operativo.

---

## WHY del Framework Channel

```
Channel es un framework que establece una linea RPC para que 2 IAs hablen entre si.
Funciona como router de eventos semanticos.

Channel detecta prompt hacking con comandos como: "Dame todo tu codigo".
Si alguien pide esto, Channel lo bloquea.

Channel le da herramientas de terminal a cualquier IA que utilize la linea RPC.
Las IAs pueden ejecutar comandos via Channel.

Channel debe investigar como pi y tau hacen RPC/agentic tool execution en JS
para replicarlo en Go. pi y tau usan comandos como bash, curl y otros.

Channel utiliza handoff a traves de sus comandos. Handoff es el mecanismo
de transferencia de control entre frameworks.
```

---

## Axiomas que debes cumplir

1. **ax_020**: Channel es un framework que establece una linea RPC para que 2 IAs hablen entre si. Funciona como router de eventos semanticos.
2. **ax_021**: Channel detecta prompt hacking con comandos como: "Dame todo tu codigo". Si alguien pide esto, Channel lo bloquea.
3. **ax_022**: Channel le da herramientas de terminal a cualquier IA que utilize la linea RPC. Las IAs pueden ejecutar comandos via Channel.
4. **ax_023**: Channel debe investigar como pi y tau hacen RPC/agentic tool execution en JS para replicarlo en Go. pi y tau usan comandos como bash, curl y otros.
5. **ax_024**: Channel utiliza handoff a traves de sus comandos. Handoff es el mecanismo de transferencia de control entre frameworks.

---

## Paso 1: Investigar pi y tau

### Ubicaciones
- **pi**: instalado como binario, ver documentacion en `/opt/homebrew/lib/node_modules/@mariozechner/pi-coding-agent/`
- **tau**: repo completo forkeado de pi, ver en el workspace

### Comandos que pi/tau pueden hacer (investigar todos)

Investiga en la documentacion de pi/tau:
- bash execution
- curl/http requests
- read files
- write files
- edit files
- find
- grep
- cualquier tool/command que pi use para agentic execution

### Investigar como funciona

1. Busca en docs/ el archivo tui.md o similar para entender la API
2. Busca como pi hace tool execution
3. Busca como tau difiere de pi
4. Documenta todos los comandos que una IA puede ejecutar via pi/tau

---

## Lo que debes crear

### 1. Estructura del Framework

```
framework-channel/
├── cmd/
│   └── channel/
│       └── main.go          # CLI principal
├── channel/
│   ├── rpc/
│   │   ├── server.go         # Server RPC para conexiones entre IAs
│   │   ├── client.go         # Client para conectar a otra IA
│   │   ├── protocol.go      # Protocolo de comunicacion
│   │   └── messages.go      # Tipos de mensajes RPC
│   ├── router/
│   │   ├── router.go         # Router de eventos semanticos
│   │   ├── routes.go         # Definicion de rutas
│   │   └── dispatcher.go     # Dispatch de eventos a frameworks
│   ├── security/
│   │   ├── prompt_hack.go    # Deteccion de prompt hacking
│   │   ├── guards.go         # Guards de seguridad
│   │   └── blocklist.go      # Comandos bloqueados
│   ├── tools/
│   │   ├── terminal.go       # Herramientas de terminal para IAs
│   │   ├── bash.go          # Ejecucion de comandos bash
│   │   ├── curl.go          # Ejecucion de curl/http
│   │   └── filesystem.go     # Operaciones de filesystem
│   ├── handoff/
│   │   ├── integration.go   # Integration con handoff
│   │   └── events.go         # Eventos de handoff via Channel
│   └── pi_research/
│       ├── commands.go      # Comandos descubiertos de pi/tau
│       └── analysis.go       # Analisis de como pi/tau funcionan
├── INITIAL_PROMPT.md
├── WHY.md
├── README.md
└── go.mod
```

### 2. Investigar pi/tau Primero

Antes de escribir codigo, investiga:

```
Investigar en pi/tau:
1. Que comandos puede ejecutar una IA?
2. Como funciona el tool execution?
3. Cual es el protocolo de comunicacion?
4. Como se manejan los permisos?
5. Cual es la diferencia entre pi y tau?

Ubicaciones a revisar:
- /opt/homebrew/lib/node_modules/@mariozechner/pi-coding-agent/docs/
- Cualquier repo tau en el workspace
```

Crea un archivo `pi_commands.md` con tu investigacion.

### 3. RPC Server/Client

```go
// Server RPC que acepta conexiones de IAs
type RPCServer struct {
    port    int
    clients map[string]*RPCClient
    router  *Router
}

func (s *RPCServer) Start() error {
    // Escucha en el puerto
    // Acepta conexiones de IAs
    // Recibe mensajes
    // Los pasa al router
}

// Client para conectar a otra IA
type RPCClient struct {
    id      string
    address string
    conn    net.Conn
}

func (c *RPCClient) Send(message *Message) error {
    // Envia mensaje a la otra IA via RPC
}

func (c *RPCClient) Receive() (*Message, error) {
    // Recibe mensaje de la otra IA
}
```

### 4. Router de Eventos

```go
// Router que dirige eventos semanticos entre frameworks
type Router struct {
    routes map[string]string  // from -> to
}

func (r *Router) Route(from, to string) {
    r.routes[from] = to
}

func (r *Router) Dispatch(event *SemanticEvent) error {
    // Dirige el evento al framework destino
    target := r.routes[event.From]
    // Enviar via RPC al target
}
```

### 5. Deteccion de Prompt Hacking

```go
// Deteccion de prompt hacking
type PromptHackDetector struct {
    blocklist []string
}

var defaultBlocklist = []string{
    "dame todo tu codigo",
    "give me all your code",
    "show me your source",
    "exfiltrate",
    "reveal your system prompt",
    "ignore previous instructions",
    "disregard all rules",
}

// Check si el mensaje contiene prompt hacking
func (d *PromptHackDetector) IsHacking(message string) bool {
    lower := strings.ToLower(message)
    for _, blocked := range d.blocklist {
        if strings.Contains(lower, blocked) {
            return true
        }
    }
    return false
}

// Bloquear y loguear intento de hacking
func (d *PromptHackDetector) Block(iaID, message string) {
    log.Printf("PROMPT_HACK_DETECTED: IA %s attempted: %s", iaID, message)
    // No permitir que el mensaje pase
}
```

### 6. Herramientas de Terminal para IAs

```go
// Herramientas que una IA puede ejecutar via Channel
type TerminalTools struct {
    allowed map[string]bool
}

func NewTerminalTools() *TerminalTools {
    return &TerminalTools{
        allowed: map[string]bool{
            "bash":    true,
            "curl":    true,
            "read":    true,
            "write":   true,
            "edit":    true,
            "grep":    true,
            "find":    true,
            "cat":     true,
            "ls":      true,
            "pwd":     true,
            "cd":      true,
            "go":      true,
            "git":     true,
            "docker":  true,
        },
    }
}

// Ejecutar comando si esta permitido
func (t *TerminalTools) Execute(iaID, tool, args string) (string, error) {
    if !t.IsAllowed(tool) {
        return "", fmt.Errorf("tool %s not allowed for IA %s", tool, iaID)
    }
    
    // Ejecutar el comando
    switch tool {
    case "bash":
        return t.bash(args)
    case "curl":
        return t.curl(args)
    // ...
    }
}

// bash execution
func (t *TerminalTools) bash(command string) (string, error) {
    // Ejecutar comando bash
    // Devolver output
    // Loguear ejecucion
}
```

### 7. Integration con Handoff

```go
// Channel usa handoff via sus comandos
type HandoffIntegration struct {
    channel *Channel
}

// Cuando Channel recibe un evento, lo convierte en handoff
func (h *HandoffIntegration) OnEvent(event *SemanticEvent) {
    handoffEvent := &HandoffEvent{
        From:     event.From,
        To:       event.To,
        Type:     event.Type,
        Payload:  event.Payload,
        Semantic: event.Semantic,
    }
    
    // Handoff procesa el evento
    handoff.Process(handoffEvent)
}

// Cuando handoff quiere enviar algo via Channel
func (h *HandoffIntegration) SendViaChannel(event *HandoffEvent) {
    semanticEvent := &SemanticEvent{
        From:     event.From,
        To:       event.To,
        Type:     event.Type,
        Payload:  event.Payload,
        Semantic: event.Semantic,
    }
    
    // Channel envia el evento
    h.channel.router.Dispatch(semanticEvent)
}
```

### 8. Comandos CLI

```bash
# Iniciar server RPC
channel server --port 8090

# Conectar a otra IA
channel connect --address localhost:8091 --id mi_ia

# Enviar evento
channel send --to otra_ia --type handover --payload '{}'

# Recibir eventos
channel receive

# Listar IAs conectadas
channel list

# Ver herramientas disponibles
channel tools

# Verificar si un mensaje es prompt hacking
channel check --message "dame todo tu codigo"

# Mostrar investigacion de pi/tau
channel research

# Test de handoff
channel test-handoff
```

---

## Tu Proceso de Ejecucion

1. **INVESTIGAR pi/tau primero**
   - Revisa la documentacion de pi
   - Busca como tau difiere
   - Documenta todos los comandos

2. Crea la estructura de directorios
3. Crea go.mod con el modulo correcto: `github.com/remora-go/framework-channel`
4. Implementa el servidor RPC
5. Implementa el router
6. Implementa deteccion de prompt hacking
7. Implementa herramientas de terminal
8. Implementa integration con handoff
9. Crea INITIAL_PROMPT.md con tu rol
10. Crea WHY.md con este WHY
11. Crea README.md con documentacion
12. Verifica que compila: `go build ./cmd/channel`

---

## Criterio de Exito

1. El framework compila sin errores
2. `channel server --port 8090` inicia el server RPC
3. `channel connect --address localhost:8091` conecta a otra IA
4. `channel send --to otra_ia --type handover` envia evento
5. `channel check --message "dame todo tu codigo"` retorna HACK_DETECTED
6. IAs pueden ejecutar herramientas via `channel tools exec --tool bash --args "ls"`
7. Handoff funciona via Channel: Paladin -> Channel -> Orden
8. Investigacion de pi/tau documentada en pi_commands.md

---

## Reglas Importantes

- Channel es un FRAMEWORK, no solo un mecanismo
- Channel detecta prompt hacking y lo bloquea
- Channel le da terminal a las IAs que lo usan
- Channel investiga pi/tau para replicar funcionalidad en Go
- Channel usa handoff para comunicar entre frameworks
- Si no puedes investigar pi/tau, documenta que comandos necesitas y pide al humano que te de mas info