package main

import "github.com/remora-go/framework-llm/llm"

// stubResponses son las respuestas determinísticas que Carolina usa
// cuando no hay ANTHROPIC_API_KEY. Permiten validar el flujo end-to-end
// sin gastar tokens.
var stubResponses = []string{
	"Hola Patricia, te escribo de Somos Rentable. Sé que las cosas a veces se complican y por eso quería contactarte directo, sin presión. Tenemos un saldo pendiente de $847.000 desde hace algunas semanas. ¿Cómo has estado? ¿Hay algo que pueda ayudarte a resolver esto esta semana?",
	"Entiendo perfectamente. Antes de hablar de números, cuéntame: ¿tu situación actual te permitiría hacer un esfuerzo único este mes, o te acomoda mejor dividirlo en algunas cuotas? Solo para ver qué camino tiene más sentido para ti.",
	"Genial. Mira, tengo dos opciones que puedo ofrecerte:\n\n• 3 cuotas de $282.333 sin recargo (próximo pago el día 5)\n• Pago único con 8% de descuento: $779.240\n\n¿Cuál te acomoda más? Si ninguna calza, dime y vemos.",
	"Perfecto. Confirmo entonces 3 cuotas de $282.333, primera vence el 5 del próximo mes. Te llegará un correo con el detalle y un link de pago en los próximos minutos. Cualquier cosa me escribes acá mismo. Gracias por la disposición, Patricia.",
}

// LLMClient es el cliente LLM compartido. Producción usa Anthropic real,
// desarrollo usa stub determinístico — sin código condicional en Carolina.
var LLMClient = llm.NewClientOrStub(stubResponses...)
