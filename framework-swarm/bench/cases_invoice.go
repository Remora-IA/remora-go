package bench

// InvoiceCase validates that a swarm can process a batch of invoices:
// validate → extract → match vendors → calculate → route approvals.
//
// Domain: financial operations
// Zones:  5 processing steps
// Expected BravoScore: ~0.85 (1 violation: unknown vendor V-UNKNOWN)
// The violation is intentional — it proves Bravo detects real problems.

import (
	"context"
	"fmt"
	"math"
	"strings"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// InvoiceCase returns a SwarmCase for invoice batch processing.
func InvoiceCase() SwarmCase {
	return SwarmCase{
		Name: "invoice-processing",
		Zones: []swarm.Zone{
			{ID: "validate_invoices", Name: "Validate Invoices", PainWeight: 0.95,
				Description: "Check required fields on all invoices"},
			{ID: "extract_data", Name: "Extract Data", PainWeight: 0.88,
				Description: "Extract amounts, vendor IDs, line items"},
			{ID: "match_vendors", Name: "Match Vendors", PainWeight: 0.80,
				Description: "Verify each vendor against the approved registry"},
			{ID: "calculate_totals", Name: "Calculate Totals", PainWeight: 0.75,
				Description: "Sum line items and verify balance"},
			{ID: "route_approvals", Name: "Route Approvals", PainWeight: 0.70,
				Description: "Route invoices to the correct approver tier"},
		},
		IdealFlow: &swarm.IdealFlow{
			Description: "Invoice Processing Automation",
			Intent:      "Validate, extract, verify, calculate, and route a batch of invoices",
			CriticalPath: []string{
				"validate_invoices", "extract_data", "match_vendors",
				"calculate_totals", "route_approvals",
			},
			CriticalVars: []string{
				"invoice_id", "total_amount", "vendor_id",
				"line_items_sum", "amounts_balanced",
				"vendor_match", "approval_level",
			},
			Rules: []swarm.VerifyRule{
				{Name: "invoice-completeness-rule",
					Description: "Invoices must have ID, VendorID, Amount, and line items",
					When:        "invoice received", Then: "validate all required fields",
					Importance: 1},
				{Name: "vendor-registry-rule",
					Description: "VendorID must exist in the approved vendor registry",
					When:        "VendorID present", Then: "check against registry",
					Importance: 1},
				{Name: "amounts-balance-rule",
					Description: "Sum of line items must equal invoice total (±$0.01)",
					When:        "line items extracted", Then: "assert sum ≈ total",
					Importance: 1},
				{Name: "approval-threshold-rule",
					Description: "Invoices > $10,000 require senior approval",
					When:        "amount > 10000", Then: "route to senior approver",
					Importance: 2},
			},
		},
		WorkFn:    invoiceWorkFn(),
		Threshold: 0.80,
	}
}

type invoice struct {
	id        string
	vendorID  string
	amount    float64
	lineItems []struct{ desc string; amt float64 }
}

var testInvoices = []invoice{
	{"INV-001", "V-ACME", 5000.00,
		[]struct{ desc string; amt float64 }{{"Software", 3000}, {"Support", 2000}}},
	{"INV-002", "V-GLOBEX", 15000.00,
		[]struct{ desc string; amt float64 }{{"Consulting", 10000}, {"Hardware", 5000}}},
	{"INV-003", "V-UNKNOWN", 3500.00, // unknown vendor → violation
		[]struct{ desc string; amt float64 }{{"Services", 3500}}},
	{"INV-004", "V-INITECH", 8750.00,
		[]struct{ desc string; amt float64 }{{"Dev", 6000}, {"Design", 2750}}},
}

var invoiceVendorRegistry = map[string]string{
	"V-ACME":    "ACME Corp",
	"V-GLOBEX":  "Globex Inc",
	"V-INITECH": "Initech LLC",
}

func invoiceWorkFn() swarm.WorkFunc {
	return func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
		tc := agent.TraceCtx()
		vars := make(map[string]any)

		switch zone.ID {
		case "validate_invoices":
			valid, ids := 0, []string{}
			for _, inv := range testInvoices {
				if inv.id != "" && inv.vendorID != "" && inv.amount > 0 && len(inv.lineItems) > 0 {
					valid++
					ids = append(ids, inv.id)
				}
			}
			vars["invoice_id"] = strings.Join(ids, ",")
			vars["validation_status"] = fmt.Sprintf("%d/%d valid", valid, len(testInvoices))
			if tc != nil {
				tc.Rule("invoice-completeness-rule", "All required fields present", nil)
				tc.Check("required_fields", "all_present",
					fmt.Sprintf("%d/%d present", valid, len(testInvoices)),
					valid == len(testInvoices))
			}

		case "extract_data":
			total, vendorIDs, itemCount := 0.0, []string{}, 0
			for _, inv := range testInvoices {
				total += inv.amount
				vendorIDs = append(vendorIDs, inv.vendorID)
				itemCount += len(inv.lineItems)
			}
			vars["total_amount"] = total
			vars["vendor_id"] = strings.Join(vendorIDs, ",")
			if tc != nil {
				tc.Event("data_extracted",
					fmt.Sprintf("total=$%.2f vendors=%d items=%d", total, len(vendorIDs), itemCount),
					nil)
			}

		case "match_vendors":
			matched, unknown := 0, []string{}
			for _, inv := range testInvoices {
				if _, ok := invoiceVendorRegistry[inv.vendorID]; ok {
					matched++
				} else {
					unknown = append(unknown, inv.vendorID)
				}
			}
			vars["vendor_match"] = fmt.Sprintf("%d/%d found", matched, len(testInvoices))
			if tc != nil {
				tc.Rule("vendor-registry-rule", "Vendor must exist in approved registry", nil)
				tc.Check("vendor-exists",
					fmt.Sprintf("%d/%d", len(testInvoices), len(testInvoices)),
					fmt.Sprintf("%d/%d", matched, len(testInvoices)),
					matched == len(testInvoices))
				if len(unknown) > 0 {
					tc.Violation("vendor_registry", "all vendors known",
						fmt.Sprintf("unknown: %v", unknown))
				}
			}

		case "calculate_totals":
			allBalanced, totalSum := true, 0.0
			for _, inv := range testInvoices {
				lineSum := 0.0
				for _, li := range inv.lineItems {
					lineSum += li.amt
				}
				totalSum += lineSum
				if math.Abs(lineSum-inv.amount) > 0.01 {
					allBalanced = false
				}
			}
			vars["line_items_sum"] = totalSum
			vars["amounts_balanced"] = allBalanced
			if tc != nil {
				tc.Rule("amounts-balance-rule", "Line items sum must equal invoice total", nil)
				status := "balanced"
				if !allBalanced {
					status = "mismatch"
				}
				tc.Check("amounts-balance", "balanced", status, allBalanced)
			}

		case "route_approvals":
			senior, standard := 0, 0
			for _, inv := range testInvoices {
				if inv.amount > 10000 {
					senior++
				} else {
					standard++
				}
			}
			vars["approval_level"] = fmt.Sprintf("%d_senior_%d_standard", senior, standard)
			vars["approver_id"] = "auto-assigned"
			if tc != nil {
				tc.Rule("approval-threshold-rule", ">$10k requires senior approval", nil)
				tc.Check("approval-routing", ">10000=senior",
					fmt.Sprintf("%d senior, %d standard", senior, standard), true)
			}
		}

		return &swarm.Result{
			Output: fmt.Sprintf("zone %s completed", zone.ID),
			Vars:   vars,
		}, nil
	}
}
