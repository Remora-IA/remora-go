package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/remora-go/framework-paladin/paladin"
)

// Ejemplo: flujo de procesamiento de órdenes
// Este ejemplo muestra cómo usar Paladin para trazar un flujo real
// y permitir que una IA entienda exactamente qué pasó y por qué.

func main() {
	// Iniciar tracing
	trace := paladin.NewTrace("order-processor")
	ctx := trace.Start()
	defer trace.Flush()

	// Simular procesamiento de órdenes
	orders := generateOrders(3)
	
	for _, order := range orders {
		processOrder(ctx, order)
	}
}

// Order representa una orden de compra
type Order struct {
	ID       string
	ClientID string
	Items    []Item
	Status   string
}

type Item struct {
	SKU      string
	Quantity int
	Price    float64
}

// generateOrders crea órdenes de ejemplo
func generateOrders(count int) []Order {
	skus := []string{"WIDGET-001", "GADGET-002", "DOOHICKY-003"}
	statuses := []string{"pending", "pending", "pending"}

	orders := make([]Order, count)
	for i := 0; i < count; i++ {
		items := []Item{
			{SKU: skus[i%len(skus)], Quantity: i + 1, Price: float64((i+1) * 10)},
		}
		orders[i] = Order{
			ID:       fmt.Sprintf("ORD-%d", time.Now().UnixNano()+int64(i)),
			ClientID: fmt.Sprintf("CLI-%d", rand.Intn(1000)),
			Items:    items,
			Status:   statuses[i],
		}
	}
	return orders
}

// processOrder maneja el flujo completo de una orden
func processOrder(ctx *paladin.Context, order Order) {
	child := ctx.Child("processOrder")
	defer child.End()

	child.Var("order.id", order.ID)
	child.Var("order.client_id", order.ClientID)
	child.Var("order.item_count", len(order.Items))

	// Validar orden
	if !validateOrder(child, order) {
		child.Decision("orden inválida", "falló validación de negocio")
		return
	}

	// Calcular total
	total := calculateTotal(child, order)
	child.Var("order.total", total)

	// Verificar crédito del cliente
	creditOk := checkCredit(child, order.ClientID, total)
	
	if !creditOk {
		child.Decision("crédito insuficiente", fmt.Sprintf("cliente %s no tiene crédito para %.2f", order.ClientID, total))
		return
	}

	// Procesar pago
	paymentOk := processPayment(child, order, total)
	if !paymentOk {
		child.ErrorMsg("pago rechazado por el gateway")
		child.Decision("orden cancelada", "pago falló")
		return
	}

	// Confirmar orden
	confirmOrder(child, order)
	child.Decision("orden completada", fmt.Sprintf("orden %s procesada exitosamente", order.ID))
}

// validateOrder valida reglas de negocio
func validateOrder(ctx *paladin.Context, order Order) bool {
	ctx.Var("validation.item_count", len(order.Items))
	
	if len(order.Items) == 0 {
		ctx.ErrorMsg("orden sin items")
		return false
	}

	for _, item := range order.Items {
		if item.Quantity <= 0 {
			ctx.ErrorMsg(fmt.Sprintf("cantidad inválida para SKU %s", item.SKU))
			return false
		}
	}

	ctx.Decision("validación exitosa", "todos los items tienen cantidad > 0")
	return true
}

// calculateTotal calcula el monto total
func calculateTotal(ctx *paladin.Context, order Order) float64 {
	child := ctx.Child("calculateTotal")
	defer child.End()

	var total float64
	for _, item := range order.Items {
		itemTotal := float64(item.Quantity) * item.Price
		child.Var(fmt.Sprintf("item.%s.subtotal", item.SKU), itemTotal)
		total += itemTotal
	}

	child.Decision("total calculado", fmt.Sprintf("suma de %d items", len(order.Items)))
	return total
}

// checkCredit verifica si el cliente tiene crédito disponible
func checkCredit(ctx *paladin.Context, clientID string, amount float64) bool {
	// Simular lógica de crédito
	limit := 1000.0
	used := rand.Float64() * 500
	
	ctx.Var("client.id", clientID)
	ctx.Var("client.credit_limit", limit)
	ctx.Var("client.credit_used", used)
	ctx.Var("client.credit_available", limit-used)
	ctx.Var("order.amount", amount)

	available := limit - used
	if available >= amount {
		ctx.Decision("crédito aprobado", fmt.Sprintf("disponible %.2f >= requerido %.2f", available, amount))
		return true
	}

	ctx.Decision("crédito denegado", fmt.Sprintf("disponible %.2f < requerido %.2f", available, amount))
	return false
}

// processPayment procesa el pago (simulado)
func processPayment(ctx *paladin.Context, order Order, total float64) bool {
	child := ctx.Child("processPayment")
	defer child.End()

	child.Var("payment.order_id", order.ID)
	child.Var("payment.amount", total)
	child.Var("payment.method", "credit_card")

	// Simular latencia de gateway
	time.Sleep(10 * time.Millisecond)

	// 90% de éxito simulado
	success := rand.Float64() > 0.1
	
	if success {
		child.Decision("pago exitoso", "gateway aprobó la transacción")
	} else {
		child.ErrorMsg("gateway respondió: insufficient_funds")
	}

	return success
}

// confirmOrder marca la orden como completada
func confirmOrder(ctx *paladin.Context, order Order) {
	ctx.Var("confirm.order_id", order.ID)
	ctx.Var("confirm.previous_status", order.Status)
	ctx.Var("confirm.new_status", "completed")
	
	order.Status = "completed"
	ctx.Decision("orden confirmada", fmt.Sprintf("status cambió de %s a completed", order.Status))
}
