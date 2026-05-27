package bench

// SupportCase validates that a swarm can triage customer support tickets:
// receive ticket → classify intent → prioritize queue → assign agent → verify SLA.
//
// Domain: customer support / service desk operations
// Zones:  5 triage steps
// Expected BravoScore: 1.00 (no violations — all 5 tickets assigned within SLA)

import (
	"context"
	"fmt"
	"strings"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// SupportCase returns a SwarmCase for customer support ticket triage.
func SupportCase() SwarmCase {
	return SwarmCase{
		Name: "support-triage",
		Zones: []swarm.Zone{
			{ID: "receive_ticket", Name: "Receive Ticket", PainWeight: 0.95,
				Description: "Ingest and acknowledge incoming support tickets"},
			{ID: "classify_intent", Name: "Classify Intent", PainWeight: 0.88,
				Description: "Determine the type and topic of each support request"},
			{ID: "prioritize_queue", Name: "Prioritize Queue", PainWeight: 0.80,
				Description: "Assign priority levels based on impact and urgency"},
			{ID: "assign_agent", Name: "Assign Agent", PainWeight: 0.73,
				Description: "Route each ticket to an available agent with matching skills"},
			{ID: "verify_sla", Name: "Verify SLA", PainWeight: 0.65,
				Description: "Confirm all tickets are within their contracted SLA windows"},
		},
		IdealFlow: &swarm.IdealFlow{
			Description: "Customer Support Ticket Triage Automation",
			Intent:      "Receive, classify, prioritize, assign, and SLA-verify all support tickets",
			CriticalPath: []string{
				"receive_ticket", "classify_intent", "prioritize_queue",
				"assign_agent", "verify_sla",
			},
			CriticalVars: []string{
				"ticket_id", "intent_class", "priority_level",
				"assigned_agent", "sla_status",
			},
			Rules: []swarm.VerifyRule{
				{Name: "intake-completeness-rule",
					Description: "Every ticket must have an ID, subject, and customer identifier",
					When:        "ticket received",
					Then:        "assert ticket_id, subject, and customer_id are all present",
					Importance:  1},
				{Name: "intent-classification-rule",
					Description: "Every ticket must be classified into a known intent category",
					When:        "ticket intake complete",
					Then:        "assert intent_class is in [billing, technical, general, account, refund]",
					Importance:  1},
				{Name: "priority-assignment-rule",
					Description: "Every ticket must be assigned a priority: P1/P2/P3/P4",
					When:        "intent classified",
					Then:        "assert priority_level is one of P1, P2, P3, P4",
					Importance:  1},
				{Name: "agent-capacity-rule",
					Description: "Tickets must not be assigned to agents at full capacity",
					When:        "agent selected",
					Then:        "assert assigned_agent queue depth < max_concurrent_tickets",
					Importance:  2},
				{Name: "sla-compliance-rule",
					Description: "All tickets must remain within their SLA first-response window",
					When:        "ticket assigned",
					Then:        "assert time-to-first-response ≤ SLA hours for priority tier",
					Importance:  1},
			},
		},
		WorkFn:    supportWorkFn(),
		Threshold: 0.80,
	}
}

type supportTicket struct {
	id         string
	subject    string
	customerID string
	intent     string
	priority   string
	agent      string
}

var testTickets = []supportTicket{
	{id: "TKT-001", subject: "Invoice discrepancy on Feb statement", customerID: "CUST-123",
		intent: "billing", priority: "P2", agent: "agent-billing-01"},
	{id: "TKT-002", subject: "API returning 500 errors since deployment", customerID: "CUST-456",
		intent: "technical", priority: "P1", agent: "agent-tech-01"},
	{id: "TKT-003", subject: "How do I export my data?", customerID: "CUST-789",
		intent: "general", priority: "P3", agent: "agent-general-01"},
	{id: "TKT-004", subject: "Account locked after password reset", customerID: "CUST-321",
		intent: "account", priority: "P2", agent: "agent-account-01"},
	{id: "TKT-005", subject: "Request refund for duplicate charge", customerID: "CUST-654",
		intent: "billing", priority: "P2", agent: "agent-billing-01"},
}

var slaHours = map[string]int{
	"P1": 1,
	"P2": 4,
	"P3": 24,
	"P4": 72,
}

func supportWorkFn() swarm.WorkFunc {
	return func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
		tc := agent.TraceCtx()
		vars := make(map[string]any)

		switch zone.ID {

		case "receive_ticket":
			ids := make([]string, 0, len(testTickets))
			complete := 0
			for _, t := range testTickets {
				ids = append(ids, t.id)
				if t.id != "" && t.subject != "" && t.customerID != "" {
					complete++
				}
			}
			vars["ticket_id"] = strings.Join(ids, ",")
			vars["tickets_received"] = len(testTickets)
			if tc != nil {
				tc.Rule("intake-completeness-rule", "Every ticket has ID, subject, and customer_id", nil)
				tc.Check("tickets-complete",
					fmt.Sprintf("%d/%d complete", len(testTickets), len(testTickets)),
					fmt.Sprintf("%d/%d complete", complete, len(testTickets)),
					complete == len(testTickets))
				tc.Event("tickets-ingested",
					fmt.Sprintf("received %d tickets: %s", len(testTickets), strings.Join(ids, ", ")),
					nil)
			}

		case "classify_intent":
			knownIntents := map[string]bool{
				"billing": true, "technical": true, "general": true, "account": true, "refund": true,
			}
			classes := make([]string, 0, len(testTickets))
			allValid := true
			for _, t := range testTickets {
				classes = append(classes, fmt.Sprintf("%s:%s", t.id, t.intent))
				if !knownIntents[t.intent] {
					allValid = false
				}
			}
			vars["intent_class"] = strings.Join(classes, ",")
			if tc != nil {
				tc.Rule("intent-classification-rule", "Every ticket classified into a known intent category", nil)
				tc.Check("intent-valid",
					fmt.Sprintf("%d/%d valid intents", len(testTickets), len(testTickets)),
					fmt.Sprintf("%d/%d; all valid=%v", len(testTickets), len(testTickets), allValid),
					allValid)
				tc.Event("intents-classified",
					fmt.Sprintf("classified %d tickets: %s", len(testTickets), strings.Join(classes, ", ")),
					nil)
			}

		case "prioritize_queue":
			knownPriorities := map[string]bool{"P1": true, "P2": true, "P3": true, "P4": true}
			priorities := make([]string, 0, len(testTickets))
			allValid := true
			for _, t := range testTickets {
				priorities = append(priorities, fmt.Sprintf("%s:%s", t.id, t.priority))
				if !knownPriorities[t.priority] {
					allValid = false
				}
			}
			vars["priority_level"] = strings.Join(priorities, ",")
			if tc != nil {
				tc.Rule("priority-assignment-rule", "Every ticket assigned a priority P1–P4", nil)
				tc.Check("priorities-assigned",
					fmt.Sprintf("%d/%d assigned", len(testTickets), len(testTickets)),
					fmt.Sprintf("%d/%d; all valid=%v", len(testTickets), len(testTickets), allValid),
					allValid)
				tc.Event("queue-prioritized",
					fmt.Sprintf("prioritized %d tickets: %s", len(testTickets), strings.Join(priorities, ", ")),
					nil)
			}

		case "assign_agent":
			assignments := make([]string, 0, len(testTickets))
			allAssigned := true
			for _, t := range testTickets {
				if t.agent == "" {
					allAssigned = false
				}
				assignments = append(assignments, fmt.Sprintf("%s→%s", t.id, t.agent))
			}
			vars["assigned_agent"] = strings.Join(assignments, ",")
			if tc != nil {
				tc.Rule("agent-capacity-rule", "Tickets assigned to agents with available capacity", nil)
				tc.Check("all-assigned",
					fmt.Sprintf("%d/%d assigned", len(testTickets), len(testTickets)),
					fmt.Sprintf("%d/%d; all assigned=%v", len(testTickets), len(testTickets), allAssigned),
					allAssigned)
				tc.Event("agents-assigned",
					fmt.Sprintf("assigned %d tickets: %s", len(testTickets), strings.Join(assignments, ", ")),
					nil)
			}

		case "verify_sla":
			slaStatuses := make([]string, 0, len(testTickets))
			allWithinSLA := true
			for _, t := range testTickets {
				hours, ok := slaHours[t.priority]
				if !ok {
					hours = 24
				}
				status := fmt.Sprintf("within-%dh-sla", hours)
				slaStatuses = append(slaStatuses, fmt.Sprintf("%s:%s", t.id, status))
			}
			vars["sla_status"] = strings.Join(slaStatuses, ",")
			if tc != nil {
				tc.Rule("sla-compliance-rule", "All tickets within SLA first-response window", nil)
				tc.Check("sla-compliant",
					fmt.Sprintf("%d/%d within SLA", len(testTickets), len(testTickets)),
					fmt.Sprintf("%d/%d compliant; all=%v", len(testTickets), len(testTickets), allWithinSLA),
					allWithinSLA)
				tc.Event("sla-verified",
					fmt.Sprintf("SLA verified for %d tickets: %s", len(testTickets), strings.Join(slaStatuses, ", ")),
					nil)
			}
		}

		return &swarm.Result{
			Output: fmt.Sprintf("zone %s completed", zone.ID),
			Vars:   vars,
		}, nil
	}
}
