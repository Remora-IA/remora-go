package bench

// ContractCase validates that a swarm can review a batch of contracts:
// extract clauses → verify parties → check dates → flag risks → approve contract.
//
// Domain: legal / contract review operations
// Zones:  5 review steps
// Expected BravoScore: ~0.85 (1 violation: missing SLA clause in CNT-003)
// The violation is intentional — it proves Bravo detects missing contract terms.

import (
	"context"
	"fmt"
	"strings"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// ContractCase returns a SwarmCase for contract review.
func ContractCase() SwarmCase {
	return SwarmCase{
		Name: "contract-review",
		Zones: []swarm.Zone{
			{ID: "extract_clauses", Name: "Extract Clauses", PainWeight: 0.95,
				Description: "Parse contract documents and extract key clauses"},
			{ID: "verify_parties", Name: "Verify Parties", PainWeight: 0.88,
				Description: "Confirm all signing parties are correctly identified and authorised"},
			{ID: "check_dates", Name: "Check Dates", PainWeight: 0.82,
				Description: "Validate effective date, expiry date, and renewal terms"},
			{ID: "flag_risks", Name: "Flag Risks", PainWeight: 0.76,
				Description: "Identify liability clauses, indemnity gaps, and compliance risks"},
			{ID: "approve_contract", Name: "Approve Contract", PainWeight: 0.70,
				Description: "Route to the correct approver tier and record approval decision"},
		},
		IdealFlow: &swarm.IdealFlow{
			Description: "Contract Review Automation",
			Intent:      "Extract, verify, validate, risk-score, and approve a batch of contracts",
			CriticalPath: []string{
				"extract_clauses", "verify_parties", "check_dates",
				"flag_risks", "approve_contract",
			},
			CriticalVars: []string{
				"contract_id", "parties_verified", "effective_date",
				"risk_count", "approval_status",
			},
			Rules: []swarm.VerifyRule{
				{Name: "party-verification-rule",
					Description: "All contracting parties must be verified before proceeding",
					When:        "contract received",
					Then:        "assert parties_verified = true for all contracts",
					Importance:  1},
				{Name: "date-validity-rule",
					Description: "Effective date must be present and not in the past",
					When:        "dates extracted",
					Then:        "assert effective_date is valid and future-dated",
					Importance:  1},
				{Name: "sla-presence-rule",
					Description: "Service contracts must include an SLA clause",
					When:        "clauses extracted",
					Then:        "assert SLA clause is present in each service contract",
					Importance:  1},
				{Name: "risk-threshold-rule",
					Description: "Contracts with more than 2 medium risks require senior review",
					When:        "risks flagged",
					Then:        "assert risk_count ≤ 2 medium risks or escalate to senior reviewer",
					Importance:  2},
				{Name: "approval-chain-rule",
					Description: "Every contract must pass through the defined approval chain",
					When:        "review complete",
					Then:        "approval_status must be approved or rejected — never empty",
					Importance:  1},
			},
		},
		WorkFn:    contractWorkFn(),
		Threshold: 0.80,
	}
}

type contractRecord struct {
	id          string
	parties     []string
	effectiveDate string
	hasSLA      bool
	risks       int
}

var testContracts = []contractRecord{
	{id: "CNT-001", parties: []string{"Acme Corp", "Globex Inc"}, effectiveDate: "2024-03-01", hasSLA: true, risks: 1},
	{id: "CNT-002", parties: []string{"Initech LLC", "Umbrella Corp"}, effectiveDate: "2024-04-15", hasSLA: true, risks: 2},
	{id: "CNT-003", parties: []string{"Globex Inc", "Initech LLC"}, effectiveDate: "2024-05-01", hasSLA: false, risks: 0}, // missing SLA
}

func contractWorkFn() swarm.WorkFunc {
	return func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
		tc := agent.TraceCtx()
		vars := make(map[string]any)

		switch zone.ID {

		case "extract_clauses":
			ids := make([]string, 0, len(testContracts))
			for _, c := range testContracts {
				ids = append(ids, c.id)
			}
			vars["contract_id"] = strings.Join(ids, ",")
			vars["clauses_extracted"] = len(testContracts)
			if tc != nil {
				tc.Event("clauses-extracted",
					fmt.Sprintf("extracted clauses from %d contracts: %s", len(testContracts), strings.Join(ids, ", ")),
					nil)
				tc.Check("extraction-complete",
					fmt.Sprintf("%d/%d", len(testContracts), len(testContracts)),
					fmt.Sprintf("%d/%d extracted", len(testContracts), len(testContracts)),
					true)
			}

		case "verify_parties":
			verified := 0
			for _, c := range testContracts {
				if len(c.parties) >= 2 {
					verified++
				}
			}
			vars["parties_verified"] = fmt.Sprintf("%d/%d", verified, len(testContracts))
			if tc != nil {
				tc.Rule("party-verification-rule", "All contracting parties verified", nil)
				tc.Check("parties-verified",
					fmt.Sprintf("%d/%d", len(testContracts), len(testContracts)),
					fmt.Sprintf("%d/%d verified", verified, len(testContracts)),
					verified == len(testContracts))
				tc.Event("parties-verified",
					fmt.Sprintf("verified parties for %d contracts", verified),
					nil)
			}

		case "check_dates":
			validDates := 0
			dates := make([]string, 0, len(testContracts))
			for _, c := range testContracts {
				if c.effectiveDate != "" {
					validDates++
					dates = append(dates, fmt.Sprintf("%s:%s", c.id, c.effectiveDate))
				}
			}
			vars["effective_date"] = strings.Join(dates, ",")
			vars["valid_date_count"] = validDates
			if tc != nil {
				tc.Rule("date-validity-rule", "Effective date present and valid for all contracts", nil)
				tc.Check("dates-valid",
					fmt.Sprintf("%d/%d", len(testContracts), len(testContracts)),
					fmt.Sprintf("%d/%d valid", validDates, len(testContracts)),
					validDates == len(testContracts))
				tc.Event("dates-validated",
					fmt.Sprintf("validated dates for %d contracts", validDates),
					nil)
			}

		case "flag_risks":
			totalRisks := 0
			missing := []string{}
			for _, c := range testContracts {
				totalRisks += c.risks
				if !c.hasSLA {
					missing = append(missing, c.id)
				}
			}
			vars["risk_count"] = fmt.Sprintf("%d total, %d missing SLA", totalRisks, len(missing))
			if tc != nil {
				tc.Rule("sla-presence-rule", "Service contracts must include an SLA clause", nil)
				tc.Rule("risk-threshold-rule", "Contracts with >2 medium risks require senior review", nil)
				tc.Check("sla-presence",
					fmt.Sprintf("%d/%d have SLA", len(testContracts), len(testContracts)),
					fmt.Sprintf("%d/%d have SLA", len(testContracts)-len(missing), len(testContracts)),
					len(missing) == 0)
				if len(missing) > 0 {
					tc.Violation("contract_completeness", "SLA clause present",
						fmt.Sprintf("SLA clause absent — contract %s missing service level terms", strings.Join(missing, ", ")))
				}
				tc.Check("risk-levels",
					"≤2 medium risks per contract",
					fmt.Sprintf("%d total risks across %d contracts", totalRisks, len(testContracts)),
					true)
			}

		case "approve_contract":
			approved := 0
			statuses := make([]string, 0, len(testContracts))
			for _, c := range testContracts {
				// CNT-003 flagged but still approved (SLA can be amended post-signature)
				status := "approved"
				if !c.hasSLA {
					status = "approved-with-conditions"
				}
				statuses = append(statuses, fmt.Sprintf("%s:%s", c.id, status))
				approved++
			}
			vars["approval_status"] = strings.Join(statuses, ",")
			if tc != nil {
				tc.Rule("approval-chain-rule", "Every contract passes through defined approval chain", nil)
				tc.Check("approvals-complete",
					fmt.Sprintf("%d/%d approved", len(testContracts), len(testContracts)),
					fmt.Sprintf("%d/%d processed", approved, len(testContracts)),
					approved == len(testContracts))
				tc.Event("contracts-approved",
					fmt.Sprintf("approval decisions recorded for %d contracts", approved),
					nil)
			}
		}

		return &swarm.Result{
			Output: fmt.Sprintf("zone %s completed", zone.ID),
			Vars:   vars,
		}, nil
	}
}
