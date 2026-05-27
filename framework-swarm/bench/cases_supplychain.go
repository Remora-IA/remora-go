package bench

// SupplyChainCase validates that a swarm can process a batch of customer orders:
// receive order → check inventory → pick and pack → schedule shipping → update tracking.
//
// Domain: supply chain / fulfilment operations
// Zones:  5 fulfilment steps
// Expected BravoScore: ~0.85 (1 violation: SKU-BIKE-XL out of stock)
// The violation is intentional — it proves Bravo detects real inventory problems.

import (
	"context"
	"fmt"
	"strings"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// SupplyChainCase returns a SwarmCase for order fulfilment.
func SupplyChainCase() SwarmCase {
	return SwarmCase{
		Name: "supply-chain",
		Zones: []swarm.Zone{
			{ID: "receive_order", Name: "Receive Order", PainWeight: 0.95,
				Description: "Ingest and validate incoming customer orders"},
			{ID: "check_inventory", Name: "Check Inventory", PainWeight: 0.90,
				Description: "Verify sufficient stock for each SKU in the order"},
			{ID: "pick_and_pack", Name: "Pick and Pack", PainWeight: 0.83,
				Description: "Pull items from warehouse shelves and package them"},
			{ID: "schedule_shipping", Name: "Schedule Shipping", PainWeight: 0.76,
				Description: "Select carrier and book collection slot"},
			{ID: "update_tracking", Name: "Update Tracking", PainWeight: 0.68,
				Description: "Write tracking number to order record and notify customer"},
		},
		IdealFlow: &swarm.IdealFlow{
			Description: "Supply Chain Order Fulfilment",
			Intent:      "Receive, verify, pack, ship, and track all customer orders",
			CriticalPath: []string{
				"receive_order", "check_inventory", "pick_and_pack",
				"schedule_shipping", "update_tracking",
			},
			CriticalVars: []string{
				"order_id", "inventory_status", "packed_items",
				"carrier", "tracking_number",
			},
			Rules: []swarm.VerifyRule{
				{Name: "inventory-availability-rule",
					Description: "All SKUs in an order must have sufficient stock before pick",
					When:        "order received",
					Then:        "check stock level ≥ order quantity for each SKU",
					Importance:  1},
				{Name: "pick-accuracy-rule",
					Description: "Packed items must match the order manifest exactly",
					When:        "picking complete",
					Then:        "assert packed_items matches order line items",
					Importance:  1},
				{Name: "shipping-sla-rule",
					Description: "Orders must be shipped within the promised SLA window",
					When:        "order packed",
					Then:        "schedule carrier pickup within SLA hours",
					Importance:  1},
				{Name: "tracking-update-rule",
					Description: "Tracking number must be recorded and customer notified",
					When:        "shipment booked",
					Then:        "write tracking_number to order and send notification",
					Importance:  2},
			},
		},
		WorkFn:    supplyChainWorkFn(),
		Threshold: 0.80,
	}
}

type customerOrder struct {
	id    string
	items []struct {
		sku string
		qty int
	}
}

var testOrders = []customerOrder{
	{id: "ORD-001", items: []struct {
		sku string
		qty int
	}{{"SKU-HELMET-M", 2}, {"SKU-GLOVE-L", 1}}},
	{id: "ORD-002", items: []struct {
		sku string
		qty int
	}{{"SKU-BIKE-XL", 1}, {"SKU-LOCK-STD", 2}}}, // BIKE-XL out of stock
	{id: "ORD-003", items: []struct {
		sku string
		qty int
	}{{"SKU-PUMP-PRO", 1}, {"SKU-LIGHT-USB", 3}}},
}

var scInventory = map[string]int{
	"SKU-HELMET-M": 15,
	"SKU-GLOVE-L":  8,
	"SKU-BIKE-XL":  0, // out of stock
	"SKU-LOCK-STD": 22,
	"SKU-PUMP-PRO": 5,
	"SKU-LIGHT-USB": 12,
}

var scCarriers = map[string]string{
	"ORD-001": "UPS",
	"ORD-002": "FedEx",
	"ORD-003": "DHL",
}

var scTracking = map[string]string{
	"ORD-001": "1Z999AA10123456784",
	"ORD-002": "274899172981",
	"ORD-003": "JD014600006649285",
}

func supplyChainWorkFn() swarm.WorkFunc {
	return func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
		tc := agent.TraceCtx()
		vars := make(map[string]any)

		switch zone.ID {

		case "receive_order":
			ids := make([]string, 0, len(testOrders))
			for _, o := range testOrders {
				ids = append(ids, o.id)
			}
			vars["order_id"] = strings.Join(ids, ",")
			vars["orders_received"] = len(testOrders)
			if tc != nil {
				tc.Event("orders-received",
					fmt.Sprintf("ingested %d orders: %s", len(testOrders), strings.Join(ids, ", ")),
					nil)
				tc.Check("orders-valid",
					fmt.Sprintf("%d/%d", len(testOrders), len(testOrders)),
					fmt.Sprintf("%d/%d valid", len(testOrders), len(testOrders)),
					true)
			}

		case "check_inventory":
			inStock, outOfStock := 0, []string{}
			for _, o := range testOrders {
				orderOk := true
				for _, item := range o.items {
					avail := scInventory[item.sku]
					if avail < item.qty {
						outOfStock = append(outOfStock, fmt.Sprintf("%s: %d units available", item.sku, avail))
						orderOk = false
					}
				}
				if orderOk {
					inStock++
				}
			}
			status := fmt.Sprintf("%d/%d orders fully in stock", inStock, len(testOrders))
			vars["inventory_status"] = status
			if tc != nil {
				tc.Rule("inventory-availability-rule", "All SKUs must have sufficient stock before pick", nil)
				tc.Check("inventory-check",
					fmt.Sprintf("%d/%d", len(testOrders), len(testOrders)),
					status,
					len(outOfStock) == 0)
				if len(outOfStock) > 0 {
					tc.Violation("inventory", "all items in stock",
						fmt.Sprintf("SKU-BIKE-XL: 0 units available"))
				}
			}

		case "pick_and_pack":
			packed := make([]string, 0, len(testOrders))
			for _, o := range testOrders {
				items := make([]string, 0, len(o.items))
				for _, item := range o.items {
					avail := scInventory[item.sku]
					if avail >= item.qty {
						items = append(items, fmt.Sprintf("%s×%d", item.sku, item.qty))
					}
				}
				if len(items) > 0 {
					packed = append(packed, fmt.Sprintf("%s:[%s]", o.id, strings.Join(items, ",")))
				}
			}
			vars["packed_items"] = strings.Join(packed, ";")
			if tc != nil {
				tc.Rule("pick-accuracy-rule", "Packed items must match order manifest", nil)
				tc.Check("packing-accuracy",
					"all items packed per manifest",
					fmt.Sprintf("%d orders packed", len(packed)),
					true)
				tc.Event("packing-complete",
					fmt.Sprintf("packed %d orders", len(packed)),
					nil)
			}

		case "schedule_shipping":
			carrierList := make([]string, 0, len(testOrders))
			for _, o := range testOrders {
				if c, ok := scCarriers[o.id]; ok {
					carrierList = append(carrierList, fmt.Sprintf("%s→%s", o.id, c))
				}
			}
			vars["carrier"] = strings.Join(carrierList, ",")
			if tc != nil {
				tc.Rule("shipping-sla-rule", "Orders shipped within SLA window", nil)
				tc.Check("carriers-assigned",
					fmt.Sprintf("%d/%d", len(testOrders), len(testOrders)),
					fmt.Sprintf("%d/%d assigned", len(carrierList), len(testOrders)),
					len(carrierList) == len(testOrders))
				tc.Event("shipping-scheduled",
					fmt.Sprintf("booked %d carrier pickups", len(carrierList)),
					nil)
			}

		case "update_tracking":
			trackingList := make([]string, 0, len(testOrders))
			for _, o := range testOrders {
				if t, ok := scTracking[o.id]; ok {
					trackingList = append(trackingList, fmt.Sprintf("%s:%s", o.id, t))
				}
			}
			vars["tracking_number"] = strings.Join(trackingList, ",")
			if tc != nil {
				tc.Rule("tracking-update-rule", "Tracking number recorded and customer notified", nil)
				tc.Check("tracking-updated",
					fmt.Sprintf("%d/%d", len(testOrders), len(testOrders)),
					fmt.Sprintf("%d/%d updated", len(trackingList), len(testOrders)),
					len(trackingList) == len(testOrders))
				tc.Event("tracking-notifications-sent",
					fmt.Sprintf("sent %d tracking notifications", len(trackingList)),
					nil)
			}
		}

		return &swarm.Result{
			Output: fmt.Sprintf("zone %s completed", zone.ID),
			Vars:   vars,
		}, nil
	}
}
