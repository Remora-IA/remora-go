package bench

// DeploymentCase validates that a swarm can run a CI/CD deployment pipeline:
// build artifacts → run tests → security scan → deploy staging → deploy production.
//
// Domain: software delivery / deployment operations
// Zones:  5 pipeline stages
// Expected BravoScore: ~0.85 (1 violation: 2 medium CVEs found)
// The CVEs are non-critical so the deploy continues but the violation is recorded.

import (
	"context"
	"fmt"
	"strings"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// DeploymentCase returns a SwarmCase for a CI/CD deployment pipeline.
func DeploymentCase() SwarmCase {
	return SwarmCase{
		Name: "deployment-pipeline",
		Zones: []swarm.Zone{
			{ID: "build_artifacts", Name: "Build Artifacts", PainWeight: 0.95,
				Description: "Compile source and produce deployable build artifacts"},
			{ID: "run_tests", Name: "Run Tests", PainWeight: 0.90,
				Description: "Execute unit, integration, and e2e test suites"},
			{ID: "security_scan", Name: "Security Scan", PainWeight: 0.85,
				Description: "Run SAST and dependency vulnerability scans"},
			{ID: "deploy_staging", Name: "Deploy Staging", PainWeight: 0.78,
				Description: "Roll out build to staging environment for validation"},
			{ID: "deploy_production", Name: "Deploy Production", PainWeight: 0.70,
				Description: "Promote validated staging build to production"},
		},
		IdealFlow: &swarm.IdealFlow{
			Description: "CI/CD Deployment Pipeline Automation",
			Intent:      "Build, test, scan, and deploy software to production safely",
			CriticalPath: []string{
				"build_artifacts", "run_tests", "security_scan",
				"deploy_staging", "deploy_production",
			},
			CriticalVars: []string{
				"build_id", "test_coverage", "vulnerabilities",
				"staging_url", "production_version",
			},
			Rules: []swarm.VerifyRule{
				{Name: "build-success-rule",
					Description: "Build must succeed before any downstream stage runs",
					When:        "pipeline triggered",
					Then:        "build_artifacts must produce a valid build_id",
					Importance:  1},
				{Name: "coverage-threshold-rule",
					Description: "Test coverage must exceed 80% before proceeding",
					When:        "tests complete",
					Then:        "assert test_coverage > 0.80",
					Importance:  1},
				{Name: "zero-critical-vuln-rule",
					Description: "Zero critical CVEs allowed; medium CVEs flagged but non-blocking",
					When:        "security scan complete",
					Then:        "assert no critical vulnerabilities found",
					Importance:  1},
				{Name: "staging-approval-rule",
					Description: "Staging deployment must be approved before production promotion",
					When:        "staging deployed",
					Then:        "require approval before deploy_production",
					Importance:  2},
				{Name: "rollback-plan-rule",
					Description: "A rollback plan must be documented before production deploy",
					When:        "production deploy initiated",
					Then:        "rollback_version must be recorded",
					Importance:  2},
			},
		},
		WorkFn:    deploymentWorkFn(),
		Threshold: 0.80,
	}
}

func deploymentWorkFn() swarm.WorkFunc {
	return func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
		tc := agent.TraceCtx()
		vars := make(map[string]any)

		switch zone.ID {

		case "build_artifacts":
			buildID := "build-20240215-a1b2c3d"
			vars["build_id"] = buildID
			vars["build_status"] = "success"
			vars["artifact_size_mb"] = 142
			if tc != nil {
				tc.Rule("build-success-rule", "Build must succeed before downstream stages run", nil)
				tc.Check("build-succeeded",
					"build_id present",
					fmt.Sprintf("build_id=%s", buildID),
					buildID != "")
				tc.Event("build-complete",
					fmt.Sprintf("artifact produced: %s (142 MB)", buildID),
					nil)
			}

		case "run_tests":
			coverage := 0.87 // 87% — above the 80% threshold
			vars["test_coverage"] = fmt.Sprintf("%.2f", coverage)
			vars["tests_passed"] = 1243
			vars["tests_failed"] = 0
			if tc != nil {
				tc.Rule("coverage-threshold-rule", "Test coverage must exceed 80%", nil)
				tc.Check("coverage-check",
					">0.80",
					fmt.Sprintf("%.0f%%", coverage*100),
					coverage > 0.80)
				tc.Event("tests-complete",
					fmt.Sprintf("1243 passed, 0 failed, coverage=%.0f%%", coverage*100),
					nil)
			}

		case "security_scan":
			// 2 medium CVEs found — non-critical so deploy continues but flagged
			cves := []string{"CVE-2024-1234", "CVE-2024-5678"}
			vars["vulnerabilities"] = strings.Join(cves, ",")
			vars["critical_cves"] = 0
			vars["medium_cves"] = 2
			if tc != nil {
				tc.Rule("zero-critical-vuln-rule", "Zero critical CVEs allowed", nil)
				tc.Check("critical-cves",
					"0 critical",
					fmt.Sprintf("0 critical, %d medium", len(cves)),
					true) // no critical CVEs → rule passes
				tc.Violation("security", "zero vulnerabilities",
					fmt.Sprintf("found 2 medium CVEs: CVE-2024-1234, CVE-2024-5678"))
			}

		case "deploy_staging":
			stagingURL := "https://staging.example.com/v1.4.2"
			vars["staging_url"] = stagingURL
			vars["staging_status"] = "healthy"
			if tc != nil {
				tc.Rule("staging-approval-rule", "Staging deployed and approved before production", nil)
				tc.Check("staging-healthy",
					"status=healthy",
					"status=healthy",
					true)
				tc.Event("staging-deployed",
					fmt.Sprintf("deployed to staging: %s", stagingURL),
					nil)
			}

		case "deploy_production":
			version := "v1.4.2"
			vars["production_version"] = version
			vars["rollback_version"] = "v1.4.1"
			vars["deployment_status"] = "success"
			if tc != nil {
				tc.Rule("rollback-plan-rule", "Rollback plan documented before production deploy", nil)
				tc.Check("rollback-documented",
					"rollback_version present",
					"rollback_version=v1.4.1",
					true)
				tc.Event("production-deployed",
					fmt.Sprintf("production updated to %s (rollback: v1.4.1)", version),
					nil)
			}
		}

		return &swarm.Result{
			Output: fmt.Sprintf("zone %s completed", zone.ID),
			Vars:   vars,
		}, nil
	}
}
