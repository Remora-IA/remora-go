package charlie

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DeployTarget identifica el ambiente al que se va a deployar.
type DeployTarget string

const (
	DeployDev  DeployTarget = "dev"
	DeployProd DeployTarget = "prod"
)

// DeployPlan resume lo que charlie deploy haria/hizo.
type DeployPlan struct {
	Mode     string // "plan" | "apply"
	Target   DeployTarget
	Service  string
	Region   string
	Project  string
	Output   string // logs combinados de make/gcloud (solo en apply)
	Blockers []string
}

// BuildDeployPlan arma un plan de deploy. La regla critica:
// - target=dev por defecto.
// - target=prod SIEMPRE bloqueado a este nivel (Charlie no deploya a prod
//   sin importar los flags). Si el humano insiste, debe usar gcloud directo
//   con su propia responsabilidad.
func BuildDeployPlan(target DeployTarget) (*DeployPlan, error) {
	plan := &DeployPlan{
		Mode:    "plan",
		Target:  target,
		Region:  deployEnvOr("REGION", "us-central1"),
		Project: deployEnvOr("PROJECT_ID", "project-ceae5831-a2c9-49aa-b1c"),
	}

	switch target {
	case DeployDev, "":
		plan.Target = DeployDev
		plan.Service = "flujo-api-dev"
	case DeployProd:
		plan.Service = "flujo-api"
		plan.Blockers = append(plan.Blockers,
			"Charlie NO deploya a produccion bajo ninguna circunstancia. "+
				"Produccion solo se actualiza cuando el humano ejecuta gcloud manualmente. "+
				"Si necesitas un deploy a dev, corre: charlie deploy --apply")
	default:
		return plan, fmt.Errorf("target desconocido: %q (usa 'dev' o 'prod')", target)
	}

	return plan, nil
}

// ApplyDeploy ejecuta el deploy a dev usando `make deploy-dev` desde la raiz
// del repo. Valida primero que el target NO sea prod.
func ApplyDeploy(target DeployTarget) (*DeployPlan, error) {
	plan, err := BuildDeployPlan(target)
	if err != nil {
		return plan, err
	}
	if len(plan.Blockers) > 0 {
		return plan, fmt.Errorf("deploy bloqueado: %s", plan.Blockers[0])
	}

	root, err := repoRoot()
	if err != nil {
		return plan, fmt.Errorf("no encuentro raiz del repo: %v", err)
	}

	cmd := exec.Command("make", "deploy-dev")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PROJECT_ID="+plan.Project,
		"REGION="+plan.Region,
	)

	out, runErr := cmd.CombinedOutput()
	plan.Output = string(out)
	plan.Mode = "apply"

	appendAudit("deploy", map[string]string{
		"target":  string(plan.Target),
		"service": plan.Service,
		"ok":      fmt.Sprintf("%t", runErr == nil),
	})

	if runErr != nil {
		return plan, fmt.Errorf("make deploy-dev fallo: %v\n--- output ---\n%s", runErr, out)
	}
	return plan, nil
}

// FormatDeployPlan produce el output humano-legible.
func FormatDeployPlan(plan *DeployPlan) string {
	var b strings.Builder
	fmt.Fprintln(&b, "=== CHARLIE DEPLOY ===")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "modo:    %s\n", plan.Mode)
	fmt.Fprintf(&b, "target:  %s\n", plan.Target)
	fmt.Fprintf(&b, "service: %s\n", plan.Service)
	fmt.Fprintf(&b, "region:  %s\n", plan.Region)
	fmt.Fprintf(&b, "project: %s\n", plan.Project)

	if len(plan.Blockers) > 0 {
		fmt.Fprintln(&b, "\nBLOQUEADO:")
		for _, blk := range plan.Blockers {
			fmt.Fprintf(&b, "  ✗ %s\n", blk)
		}
		return b.String()
	}

	if plan.Mode == "plan" {
		fmt.Fprintln(&b, "\nsiguiente: charlie deploy --apply")
		return b.String()
	}

	if plan.Output != "" {
		fmt.Fprintln(&b, "\n--- output make deploy-dev ---")
		// Truncar si es muy largo.
		out := plan.Output
		if len(out) > 4000 {
			out = "..." + out[len(out)-4000:]
		}
		fmt.Fprintln(&b, out)
	}
	fmt.Fprintln(&b, "\n✅ Deploy a dev completado.")
	return b.String()
}

// repoRoot devuelve la raiz del repo. Usa la constante RepoRoot pero permite
// override via REMORA_ROOT para tests.
func repoRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv("REMORA_ROOT")); v != "" {
		return v, nil
	}
	if _, err := os.Stat(RepoRoot); err == nil {
		return RepoRoot, nil
	}
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func deployEnvOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
