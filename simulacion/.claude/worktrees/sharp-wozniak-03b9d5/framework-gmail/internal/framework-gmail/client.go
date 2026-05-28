package frameworkgmail

import (
	"time"

	"framework-framework-gmail/internal/paladin"
)

// Client es el cliente principal del framework.
type Client struct {
	trace *paladin.Trace
	ctx   *paladin.Context
}

// New crea un nuevo cliente.
func New() *Client {
	return &Client{}
}

// NewWithTrace crea un cliente con tracing activo.
func NewWithTrace(name string) *Client {
	trace := paladin.NewTrace(name)
	ctx := trace.Start()
	return &Client{trace: trace, ctx: ctx}
}

// Flush guarda el trace actual.
func (c *Client) Flush() {
	if c.trace != nil {
		c.trace.Flush()
	}
}

// Process Procesa datos según el flujo del framework
func (c *Client) Process() (interface{}, error) {
	childCtx := c.ctx.Child("Process")
	defer childCtx.End()
	childCtx.Var("timestamp", time.Now().Format(time.RFC3339))
	childCtx.Decision("ejecutando-Process", "Procesa datos según el flujo del framework")
	return map[string]interface{}{"status": "ok", "method": "Process"}, nil
}

// Status Muestra el estado actual del framework
func (c *Client) Status() (interface{}, error) {
	childCtx := c.ctx.Child("Status")
	defer childCtx.End()
	childCtx.Var("timestamp", time.Now().Format(time.RFC3339))
	childCtx.Decision("ejecutando-Status", "Muestra el estado actual del framework")
	return map[string]interface{}{"status": "ok", "method": "Status"}, nil
}

// Validate Valida datos de entrada
func (c *Client) Validate() (interface{}, error) {
	childCtx := c.ctx.Child("Validate")
	defer childCtx.End()
	childCtx.Var("timestamp", time.Now().Format(time.RFC3339))
	childCtx.Decision("ejecutando-Validate", "Valida datos de entrada")
	return map[string]interface{}{"status": "ok", "method": "Validate"}, nil
}
