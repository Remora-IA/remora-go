# IDEAL FLOW - Procesamiento de pedido con reglas de descuento, envío y stock

**Generated:** 2026-05-27T10:28:13-04:00

## Verbalización del CTO

El flujo ideal es el siguiente:
Primero validamos que todos los items del carrito existan y tengan stock suficiente. Si alguno falla, lo rechazamos y seguimos con los demás.
Una vez tenemos el subtotal, si hay cupón, aplicamos el descuento solo sobre los productos de la categoría que corresponde (en este caso electrónica).
Luego calculamos envío: si el monto después del descuento supera los $100, el envío debe ser gratis, excepto si usamos cupón (en ese caso cobramos envío).
Después calculamos impuesto sobre el precio ya con descuento.
Finalmente sumamos todo para obtener el total y actualizamos el stock del producto correcto, no del que esté en la posición del mapa.

Los puntos críticos son: que el descuento por categoría funcione, que no se aplique doble descuento, que el envío gratis respete la regla del cupón, y que el stock se descuente del producto real pedido y no de uno aleatorio del mapa.

## Intent

Procesar pedidos respetando estrictamente las reglas de cupones por categoría, política de envío según uso de cupón, y actualización correcta de inventario. Evitar errores silenciosos en descuentos y stock es prioridad máxima.

## Reglas de Negocio

### Descuento por categoría (Importancia: 1)
El cupón TECH15 solo debe aplicarse a productos de categoría 'electronica'

**Entonces:** El descuento se calcula solo sobre laptop, mouse y hdmi. Total descuento ≈ $195

### Sin doble descuento (Importancia: 1)
El descuento debe aplicarse exactamente una sola vez sobre el subtotal

**Entonces:** precio_con_descuento = subtotal - descuento_calculado. No multiplicar el subtotal

### Envío y cupón son mutuamente exclusivos (Importancia: 1)
Si se usó cupón, NO debe aplicarse envío gratis aunque supere los $100

**Cuando:** cuando hay descuento > 0
**Entonces:** costo_envio > 0

### Actualización correcta de stock (Importancia: 1)
El stock debe descontarse del producto realmente comprado, no según orden del mapa

**Entonces:** producto_id actualizado debe coincidir con el item procesado

## Variables Críticas

- `precio_con_descuento`
- `costo_envio`
- `descuento_calculado`
- `producto_afectado`
- `stock_despues`

## Critical Path Esperado

1. `procesarPedido`
2. `calcularSubtotal`
3. `aplicarCupon`
4. `calcularEnvio`
5. `actualizarStock`
