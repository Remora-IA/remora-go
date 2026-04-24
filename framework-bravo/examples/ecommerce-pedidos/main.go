package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"framework-bravo/frameworkbravo"
)

type Producto struct {
	ID        string
	Nombre    string
	Precio    float64
	Stock     int
	Categoria string
}

type ItemCarrito struct {
	ProductoID string
	Cantidad  int
}

type Cupon struct {
	Codigo     string
	Porcentaje float64
	Categoria  string
}

type Pedido struct {
	Items         []ItemCarrito
	Cupon         *Cupon
	DireccionPais string
}

type ResumenPedido struct {
	Subtotal        float64
	Descuento       float64
	CostoEnvio      float64
	Impuesto        float64
	Total           float64
	EnvioGratis     bool
	ItemsProcesados int
}

var catalogoProductos = map[string]*Producto{
	"LAPTOP-01": {ID: "LAPTOP-01", Nombre: "Laptop Pro 15", Precio: 1200.00, Stock: 10, Categoria: "electronica"},
	"MOUSE-01":  {ID: "MOUSE-01", Nombre: "Mouse Inalámbrico", Precio: 25.00, Stock: 150, Categoria: "electronica"},
	"CAMISETA-01": {ID: "CAMISETA-01", Nombre: "Camiseta Algodón", Precio: 19.99, Stock: 500, Categoria: "ropa"},
	"LIBRO-01":  {ID: "LIBRO-01", Nombre: "Clean Code", Precio: 35.00, Stock: 45, Categoria: "libros"},
	"HDMI-01":   {ID: "HDMI-01", Nombre: "Cable HDMI 2m", Precio: 12.50, Stock: 300, Categoria: "electronica"},
}

var costoEnvioPorPais = map[string]float64{
	"US": 5.99, "MX": 8.50, "ES": 12.00, "CO": 15.00, "AR": 18.00,
}

var impuestoPorPais = map[string]float64{
	"US": 0.08, "MX": 0.16, "ES": 0.21, "CO": 0.19, "AR": 0.21,
}

func main() {
	trace := frameworkbravo.NewTrace("EcommercePedidos")
	defer trace.Flush()

	ctx := trace.Start()
	defer ctx.End()

	ctx.Var("version", "2.1.0")
	ctx.Var("ambiente", "produccion")
	ctx.Var("total_productos_catalogo", len(catalogoProductos))

	ideal := frameworkbravo.NewIdealFlow("Procesamiento de pedido con reglas de descuento, envío y stock").
		SetVerbalization(`El flujo ideal es el siguiente:
Primero validamos que todos los items del carrito existan y tengan stock suficiente. Si alguno falla, lo rechazamos y seguimos con los demás.
Una vez tenemos el subtotal, si hay cupón, aplicamos el descuento solo sobre los productos de la categoría que corresponde (en este caso electrónica).
Luego calculamos envío: si el monto después del descuento supera los $100, el envío debe ser gratis, excepto si usamos cupón (en ese caso cobramos envío).
Después calculamos impuesto sobre el precio ya con descuento.
Finalmente sumamos todo para obtener el total y actualizamos el stock del producto correcto, no del que esté en la posición del mapa.

Los puntos críticos son: que el descuento por categoría funcione, que no se aplique doble descuento, que el envío gratis respete la regla del cupón, y que el stock se descuente del producto real pedido y no de uno aleatorio del mapa.`).
		SetIntent("Procesar pedidos respetando estrictamente las reglas de cupones por categoría, " +
			"política de envío según uso de cupón, y actualización correcta de inventario. " +
			"Evitar errores silenciosos en descuentos y stock es prioridad máxima.").
		AddRule("Descuento por categoría",
			"El cupón TECH15 solo debe aplicarse a productos de categoría 'electronica'",
			"El descuento se calcula solo sobre laptop, mouse y hdmi. Total descuento ≈ $195").
		AddRule("Sin doble descuento",
			"El descuento debe aplicarse exactamente una sola vez sobre el subtotal",
			"precio_con_descuento = subtotal - descuento_calculado. No multiplicar el subtotal").
		AddRuleWithWhen("Envío y cupón son mutuamente exclusivos",
			"Si se usó cupón, NO debe aplicarse envío gratis aunque supere los $100",
			"cuando hay descuento > 0",
			"costo_envio > 0").
		AddRule("Actualización correcta de stock",
			"El stock debe descontarse del producto realmente comprado, no según orden del mapa",
			"producto_id actualizado debe coincidir con el item procesado").
		AddCriticalVar("precio_con_descuento").
		AddCriticalVar("costo_envio").
		AddCriticalVar("descuento_calculado").
		AddCriticalVar("producto_afectado").
		AddCriticalVar("stock_despues").
		SetCriticalPath("procesarPedido", "calcularSubtotal", "aplicarCupon", "calcularEnvio", "actualizarStock")

	if err := ideal.Save("."); err != nil {
		ctx.Error(err)
	}
	trace.ReloadIdealFlow()

	pedido := Pedido{
		Items: []ItemCarrito{
			{ProductoID: "LAPTOP-01", Cantidad: 1},
			{ProductoID: "MOUSE-01", Cantidad: 3},
			{ProductoID: "CAMISETA-01", Cantidad: 2},
			{ProductoID: "LIBRO-01", Cantidad: 1},
			{ProductoID: "HDMI-01", Cantidad: 2},
		},
		Cupon: &Cupon{Codigo: "TECH15", Porcentaje: 15.0, Categoria: "electronica"},
		DireccionPais: "MX",
	}

	ctx.Var("pedido_items", len(pedido.Items))
	ctx.Var("pedido_cupon", pedido.Cupon.Codigo)
	ctx.Var("pedido_pais", pedido.DireccionPais)
	ctx.Decision("pedido creado", "cliente en MX con cupón TECH15 para electrónica")

	resumen := procesarPedido(ctx, pedido)

	ctx.Var("resultado_total", resumen.Total)
	ctx.Var("resultado_envio_gratis", resumen.EnvioGratis)
	ctx.Var("resultado_items_procesados", resumen.ItemsProcesados)

	fmt.Printf("\n=== RESUMEN FINAL ===\n")
	fmt.Printf("Subtotal:    $%.2f\n", resumen.Subtotal)
	fmt.Printf("Descuento:   -$%.2f\n", resumen.Descuento)
	fmt.Printf("Envío:       $%.2f (gratis: %v)\n", resumen.CostoEnvio, resumen.EnvioGratis)
	fmt.Printf("Impuesto:    $%.2f\n", resumen.Impuesto)
	fmt.Printf("TOTAL:       $%.2f\n", resumen.Total)

	frameworkbravo.PrintVerificationInstructions()
}

func procesarPedido(parent *frameworkbravo.Context, pedido Pedido) ResumenPedido {
	ctx := parent.Child("procesarPedido")
	defer ctx.End()

	ctx.Decision("inicio procesamiento", "pedido contiene 5 items, cupón por categoría y dirección MX")

	subtotal, itemsOk := calcularSubtotal(ctx, pedido.Items)
	ctx.Var("subtotal_calculado", subtotal)
	ctx.Var("items_validos", itemsOk)

	descuento := 0.0
	precioConDescuento := subtotal
	if pedido.Cupon != nil {
		descuento, precioConDescuento = aplicarCupon(ctx, pedido.Items, pedido.Cupon, subtotal)
	}

	ctx.Var("descuento_total", descuento)
	ctx.Var("precio_con_descuento", precioConDescuento)

	costoEnvio, envioGratis := calcularEnvio(ctx, precioConDescuento, pedido.DireccionPais, descuento)
	impuesto := calcularImpuesto(ctx, precioConDescuento, pedido.DireccionPais)
	total := calcularTotal(ctx, precioConDescuento, costoEnvio, impuesto)
	actualizarStock(ctx, pedido.Items)

	ctx.Decision("pedido completado", fmt.Sprintf("total final calculado en $%.2f con %d items procesados", total, itemsOk))

	return ResumenPedido{
		Subtotal: subtotal, Descuento: descuento, CostoEnvio: costoEnvio,
		Impuesto: impuesto, Total: total, EnvioGratis: envioGratis, ItemsProcesados: itemsOk,
	}
}

func calcularSubtotal(parent *frameworkbravo.Context, items []ItemCarrito) (float64, int) {
	ctx := parent.Child("calcularSubtotal")
	defer ctx.End()

	subtotal := 0.0
	itemsValidos := 0

	for _, item := range items {
		resultado := procesarItem(ctx, item)
		if resultado >= 0 {
			subtotal += resultado
			itemsValidos++
		}
	}

	ctx.Var("subtotal_final", subtotal)
	ctx.Var("items_validos", itemsValidos)
	ctx.Decision("subtotal calculado", fmt.Sprintf("%d items válidos → $%.2f", itemsValidos, subtotal))

	return subtotal, itemsValidos
}

func procesarItem(parent *frameworkbravo.Context, item ItemCarrito) float64 {
	ctx := parent.Child("procesarItem")
	defer ctx.End()

	ctx.Var("producto_id", item.ProductoID)
	ctx.Var("cantidad_solicitada", item.Cantidad)

	producto, existe := catalogoProductos[item.ProductoID]
	if !existe {
		ctx.ErrorMsg(fmt.Sprintf("producto %s no encontrado en catálogo", item.ProductoID))
		return -1
	}

	ctx.Var("producto_nombre", producto.Nombre)
	ctx.Var("producto_precio", producto.Precio)
	ctx.Var("producto_stock", producto.Stock)

	if item.Cantidad > producto.Stock {
		ctx.ErrorMsg(fmt.Sprintf("stock insuficiente: pedido %d, disponible %d", item.Cantidad, producto.Stock))
		ctx.Decision("rechazar_item", "cantidad solicitada excede stock disponible")
		return -1
	}

	lineTotal := producto.Precio * float64(item.Cantidad)
	ctx.Var("line_total", lineTotal)
	ctx.Decision("item_aceptado", fmt.Sprintf("%s × %d = $%.2f", producto.Nombre, item.Cantidad, lineTotal))

	return lineTotal
}

func aplicarCupon(parent *frameworkbravo.Context, items []ItemCarrito, cupon *Cupon, subtotal float64) (float64, float64) {
	ctx := parent.Child("aplicarCupon")
	defer ctx.End()

	ctx.Var("cupon_codigo", cupon.Codigo)
	ctx.Var("cupon_porcentaje", cupon.Porcentaje)
	ctx.Var("cupon_categoria", cupon.Categoria)
	ctx.Var("subtotal_original", subtotal)

	time.Sleep(15 * time.Millisecond)

	descuentoTotal := 0.0

	if cupon.Categoria == "" {
		descuentoTotal = subtotal * (cupon.Porcentaje / 100)
		ctx.Decision("aplicar_descuento_global", fmt.Sprintf("%.1f%% sobre todo el pedido", cupon.Porcentaje))
	} else {
		descuentoTotal = calcularDescuentoPorCategoria(ctx, items, cupon)
		ctx.Decision("aplicar_descuento_por_categoria", fmt.Sprintf("solo categoría '%s'", cupon.Categoria))
	}

	descuentoTotal = math.Round(descuentoTotal*100) / 100
	precioConDescuento := subtotal - descuentoTotal
	precioConDescuento = precioConDescuento * (1 - cupon.Porcentaje/100)

	ctx.Var("descuento_calculado", descuentoTotal)
	ctx.Var("precio_con_descuento", precioConDescuento)
	ctx.Decision("precio_tras_descuento", fmt.Sprintf("$%.2f - $%.2f = $%.2f", subtotal, descuentoTotal, precioConDescuento))

	return descuentoTotal, precioConDescuento
}

func calcularDescuentoPorCategoria(parent *frameworkbravo.Context, items []ItemCarrito, cupon *Cupon) float64 {
	ctx := parent.Child("calcularDescuentoPorCategoria")
	defer ctx.End()

	ctx.Var("categoria_objetivo", cupon.Categoria)

	totalCategoria := 0.0
	itemsAplicables := 0

	for _, item := range items {
		producto, existe := catalogoProductos[item.ProductoID]
		if !existe {
			continue
		}

		esCategoriaValida := strings.EqualFold(producto.Categoria, cupon.Categoria)
		ctx.Var(fmt.Sprintf("item_%s_aplica", item.ProductoID), esCategoriaValida)

		if esCategoriaValida {
			lineTotal := producto.Precio * float64(item.Cantidad)
			totalCategoria += lineTotal
			itemsAplicables++
		}
	}

	descuento := totalCategoria * (cupon.Porcentaje / 100)

	ctx.Var("total_categoria", totalCategoria)
	ctx.Var("items_aplicables", itemsAplicables)
	ctx.Var("descuento_categoria", descuento)

	ctx.Decision("descuento_por_categoria_calculado",
		fmt.Sprintf("%.1f%% sobre $%.2f (%d items de %s)", cupon.Porcentaje, totalCategoria, itemsAplicables, cupon.Categoria))

	return descuento
}

func calcularEnvio(parent *frameworkbravo.Context, precioConDescuento float64, pais string, descuento float64) (float64, bool) {
	ctx := parent.Child("calcularEnvio")
	defer ctx.End()

	ctx.Var("precio_base_envio", precioConDescuento)
	ctx.Var("pais_destino", pais)
	ctx.Var("descuento_aplicado", descuento)

	costoBase, existe := costoEnvioPorPais[pais]
	if !existe {
		ctx.ErrorMsg(fmt.Sprintf("país %s sin costo de envío definido", pais))
		costoBase = 25.00
		ctx.Decision("costo_envio_fallback", "país no encontrado en tabla")
	}

	ctx.Var("costo_base_envio", costoBase)

	umbralEnvioGratis := 100.0
	ctx.Var("umbral_envio_gratis", umbralEnvioGratis)

	envioGratis := precioConDescuento > umbralEnvioGratis
	ctx.Var("envio_gratis_evaluacion", envioGratis)

	costoFinal := 0.0
	if envioGratis {
		costoFinal = 0.0
		ctx.Decision("envio_gratis_aplicado", fmt.Sprintf("$%.2f > $%.2f", precioConDescuento, umbralEnvioGratis))
	} else {
		costoFinal = costoBase
		ctx.Decision("envio_cobrado", fmt.Sprintf("$%.2f <= $%.2f", precioConDescuento, umbralEnvioGratis))
	}

	if descuento > 0 {
		costoFinal = costoBase
		ctx.Decision("ajuste_por_cupon", "se cobra envío porque se usó cupón")
	}

	ctx.Var("costo_envio_final", costoFinal)

	time.Sleep(80 * time.Millisecond)

	return costoFinal, envioGratis
}

func calcularImpuesto(parent *frameworkbravo.Context, precioConDescuento float64, pais string) float64 {
	ctx := parent.Child("calcularImpuesto")
	defer ctx.End()

	ctx.Var("base_imponible", precioConDescuento)
	ctx.Var("pais", pais)

	tasa, existe := impuestoPorPais[pais]
	if !existe {
		ctx.ErrorMsg(fmt.Sprintf("país %s sin tasa de impuesto", pais))
		tasa = 0.10
		ctx.Decision("impuesto_default", "tasa no encontrada → 10%")
	}

	impuesto := precioConDescuento * tasa
	impuesto = math.Round(impuesto*100) / 100

	ctx.Var("tasa_impuesto", tasa)
	ctx.Var("impuesto_calculado", impuesto)
	ctx.Decision("impuesto_aplicado", fmt.Sprintf("$%.2f × %.0f%% = $%.2f", precioConDescuento, tasa*100, impuesto))

	return impuesto
}

func calcularTotal(parent *frameworkbravo.Context, precioConDescuento float64, costoEnvio float64, impuesto float64) float64 {
	ctx := parent.Child("calcularTotal")
	defer ctx.End()

	total := precioConDescuento + costoEnvio + impuesto
	total = math.Round(total*100) / 100

	ctx.Var("precio_con_descuento", precioConDescuento)
	ctx.Var("costo_envio", costoEnvio)
	ctx.Var("impuesto", impuesto)
	ctx.Var("total_final", total)

	ctx.Decision("total_final_calculado", fmt.Sprintf("$%.2f + $%.2f + $%.2f = $%.2f", precioConDescuento, costoEnvio, impuesto, total))

	return total
}

func actualizarStock(parent *frameworkbravo.Context, items []ItemCarrito) {
	ctx := parent.Child("actualizarStock")
	defer ctx.End()

	time.Sleep(100 * time.Millisecond)

	productKeys := make([]string, 0, len(catalogoProductos))
	for k := range catalogoProductos {
		productKeys = append(productKeys, k)
	}

	for i, item := range items {
		ctx.Var(fmt.Sprintf("procesando_item_%d", i), item.ProductoID)
		ctx.Var(fmt.Sprintf("cantidad_a_descontar_%d", i), item.Cantidad)

		if i < len(productKeys) {
			productoAfectado := catalogoProductos[productKeys[i]]
			stockAntes := productoAfectado.Stock
			productoAfectado.Stock -= item.Cantidad

			ctx.Var(fmt.Sprintf("producto_afectado_%d", i), productoAfectado.ID)
			ctx.Var(fmt.Sprintf("stock_antes_%d", i), stockAntes)
			ctx.Var(fmt.Sprintf("stock_despues_%d", i), productoAfectado.Stock)

			ctx.Decision(fmt.Sprintf("stock_actualizado_item_%d", i),
				fmt.Sprintf("%s: %d → %d (se procesaba %s)", productoAfectado.ID, stockAntes, productoAfectado.Stock, item.ProductoID))
		}
	}

	ctx.Var("stock_actualizado", true)
	ctx.Decision("actualizacion_stock_finalizada", "se procesaron todos los items")
}