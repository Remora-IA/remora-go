package bench

// HRCase validates that a swarm can onboard new employees end-to-end:
// create account → send welcome → assign team → schedule training → provision tools.
//
// Domain: HR / people operations
// Zones:  5 onboarding steps
// Expected BravoScore: 1.00 (no violations — all employees fully onboarded)

import (
	"context"
	"fmt"
	"strings"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// HRCase returns a SwarmCase for employee onboarding.
func HRCase() SwarmCase {
	return SwarmCase{
		Name: "hr-onboarding",
		Zones: []swarm.Zone{
			{ID: "create_account", Name: "Create Account", PainWeight: 0.95,
				Description: "Provision identity and system credentials for new hire"},
			{ID: "send_welcome", Name: "Send Welcome", PainWeight: 0.88,
				Description: "Deliver welcome email with first-day instructions"},
			{ID: "assign_team", Name: "Assign Team", PainWeight: 0.80,
				Description: "Add new hire to the correct team and reporting structure"},
			{ID: "schedule_training", Name: "Schedule Training", PainWeight: 0.73,
				Description: "Book mandatory onboarding training sessions"},
			{ID: "provision_tools", Name: "Provision Tools", PainWeight: 0.65,
				Description: "Grant access to required software and services"},
		},
		IdealFlow: &swarm.IdealFlow{
			Description: "Employee Onboarding Automation",
			Intent:      "Fully onboard new employees: account, welcome, team, training, tools",
			CriticalPath: []string{
				"create_account", "send_welcome", "assign_team",
				"schedule_training", "provision_tools",
			},
			CriticalVars: []string{
				"employee_id", "welcome_sent", "team_name",
				"training_date", "tools_provisioned",
			},
			Rules: []swarm.VerifyRule{
				{Name: "account-creation-rule",
					Description: "Every new hire must receive a system account before day one",
					When:        "employee record created",
					Then:        "create account and assign employee_id",
					Importance:  1},
				{Name: "welcome-rule",
					Description: "Welcome email must be sent within 24 hours of account creation",
					When:        "account created",
					Then:        "send_welcome_email with first-day details",
					Importance:  1},
				{Name: "team-assignment-rule",
					Description: "Every employee must be assigned to exactly one team",
					When:        "onboarding started",
					Then:        "team_name must be populated",
					Importance:  1},
				{Name: "training-schedule-rule",
					Description: "Mandatory training must be scheduled within the first week",
					When:        "team assigned",
					Then:        "training_date must be within 7 days of start",
					Importance:  2},
				{Name: "tool-provisioning-rule",
					Description: "All required tools must be provisioned before start date",
					When:        "onboarding complete",
					Then:        "tools_provisioned must list all required accesses",
					Importance:  2},
			},
		},
		WorkFn:    hrWorkFn(),
		Threshold: 0.80,
	}
}

type newEmployee struct {
	id        string
	name      string
	team      string
	startDate string
}

var testEmployees = []newEmployee{
	{id: "EMP-001", name: "Alice Chen", team: "engineering", startDate: "2024-02-01"},
	{id: "EMP-002", name: "Bob Patel", team: "product", startDate: "2024-02-01"},
	{id: "EMP-003", name: "Carol Smith", team: "design", startDate: "2024-02-05"},
}

func hrWorkFn() swarm.WorkFunc {
	return func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
		tc := agent.TraceCtx()
		vars := make(map[string]any)

		switch zone.ID {

		case "create_account":
			ids := make([]string, 0, len(testEmployees))
			for _, emp := range testEmployees {
				ids = append(ids, emp.id)
			}
			vars["employee_id"] = strings.Join(ids, ",")
			vars["accounts_created"] = len(testEmployees)
			if tc != nil {
				tc.Rule("account-creation-rule", "Every new hire receives a system account", nil)
				tc.Check("accounts-provisioned",
					fmt.Sprintf("%d/%d", len(testEmployees), len(testEmployees)),
					fmt.Sprintf("%d/%d created", len(testEmployees), len(testEmployees)),
					true)
				tc.Event("accounts-created",
					fmt.Sprintf("provisioned %d accounts: %s", len(testEmployees), strings.Join(ids, ", ")),
					nil)
			}

		case "send_welcome":
			sent := 0
			for range testEmployees {
				sent++
			}
			vars["welcome_sent"] = fmt.Sprintf("%d/%d", sent, len(testEmployees))
			if tc != nil {
				tc.Rule("welcome-rule", "Welcome email sent within 24h of account creation", nil)
				tc.Check("welcome-emails-sent",
					fmt.Sprintf("%d/%d", len(testEmployees), len(testEmployees)),
					fmt.Sprintf("%d/%d sent", sent, len(testEmployees)),
					sent == len(testEmployees))
				tc.Event("welcome-emails-dispatched",
					fmt.Sprintf("sent welcome emails to %d new hires", sent),
					nil)
			}

		case "assign_team":
			assignments := make([]string, 0, len(testEmployees))
			for _, emp := range testEmployees {
				assignments = append(assignments, fmt.Sprintf("%s→%s", emp.id, emp.team))
			}
			vars["team_name"] = strings.Join(assignments, ",")
			if tc != nil {
				tc.Rule("team-assignment-rule", "Every employee assigned to exactly one team", nil)
				tc.Check("team-assignments-complete",
					fmt.Sprintf("%d/%d", len(testEmployees), len(testEmployees)),
					fmt.Sprintf("%d/%d assigned", len(testEmployees), len(testEmployees)),
					true)
				tc.Event("teams-assigned",
					fmt.Sprintf("assigned %d employees: %s", len(testEmployees), strings.Join(assignments, ", ")),
					nil)
			}

		case "schedule_training":
			trainingDates := make([]string, 0, len(testEmployees))
			for _, emp := range testEmployees {
				trainingDate := emp.startDate // schedule on start date
				trainingDates = append(trainingDates, fmt.Sprintf("%s:%s", emp.id, trainingDate))
			}
			vars["training_date"] = strings.Join(trainingDates, ",")
			if tc != nil {
				tc.Rule("training-schedule-rule", "Mandatory training scheduled within first week", nil)
				tc.Check("training-scheduled",
					fmt.Sprintf("%d/%d", len(testEmployees), len(testEmployees)),
					fmt.Sprintf("%d/%d scheduled", len(testEmployees), len(testEmployees)),
					true)
				tc.Event("training-sessions-booked",
					fmt.Sprintf("scheduled %d onboarding training sessions", len(testEmployees)),
					nil)
			}

		case "provision_tools":
			toolSets := map[string][]string{
				"engineering": {"github", "jira", "slack", "datadog"},
				"product":     {"jira", "slack", "figma", "notion"},
				"design":      {"figma", "slack", "notion", "miro"},
			}
			provisioned := make([]string, 0, len(testEmployees))
			for _, emp := range testEmployees {
				tools := toolSets[emp.team]
				provisioned = append(provisioned, fmt.Sprintf("%s:[%s]", emp.id, strings.Join(tools, ",")))
			}
			vars["tools_provisioned"] = strings.Join(provisioned, ";")
			if tc != nil {
				tc.Rule("tool-provisioning-rule", "All required tools provisioned before start date", nil)
				tc.Check("tools-provisioned",
					fmt.Sprintf("%d/%d", len(testEmployees), len(testEmployees)),
					fmt.Sprintf("%d/%d provisioned", len(testEmployees), len(testEmployees)),
					true)
				tc.Event("tools-granted",
					fmt.Sprintf("provisioned tool access for %d employees", len(testEmployees)),
					nil)
			}
		}

		return &swarm.Result{
			Output: fmt.Sprintf("zone %s completed", zone.ID),
			Vars:   vars,
		}, nil
	}
}
