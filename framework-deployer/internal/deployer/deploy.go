// Package deployer encapsula el deploy de remora-go a Cloud Run.
//
// Reglas no negociables:
//   - Default: deploya a "flujo-api-dev" en us-central1.
//   - Refuses target=prod en TODOS los casos. Produccion solo la actualiza
//     el humano manualmente.
//   - NO genera commits, tags ni push de git. Es ortogonal a Charlie.
//   - Audita cada deploy en framework-deployer/temp/applied.jsonl.
package deployer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Target identifica el ambiente.
type Target string

const (
	Dev  Target = "dev"
	Prod Target = "prod"
)

// Plan resume el deploy planificado o ejecutado.
type Plan struct {
	Mode     string `json:"mode"` // "plan" | "apply"
	Target   Target `json:"target"`
	Service  string `json:"service"`
	Region   string `json:"region"`
	Project  string `json:"project"`
	Output   string `json:"output,omitempty"`
	Blockers []string `json:"blockers,omitempty"`
}

// Build devuelve el plan sin ejecutar nada. Aplica la regla de prod.
func Build(target Target) (*Plan, error) {
	plan := &Plan{
		Mode:    "plan",
		Target:  target,
		Region:  envOr("REGION", "us-central1"),
		Project: envOr("PROJECT_ID", "project-ceae5831-a2c9-49aa-b1c"),
	}

	switch target {
	case Dev, "":
		plan.Target = Dev
		plan.Service = envOr("DEV_SERVICE", "flujo-api-dev")
	case Prod:
		plan.Service = "flujo-api"
		plan.Blockers = append(plan.Blockers,
			"framework-deployer NO toca produccion. Solo el humano puede ejecutar "+
				"manualmente 'gcloud run deploy flujo-api ...' asumiendo su responsabilidad.")
	default:
		return plan, fmt.Errorf("target desconocido: %q (usa 'dev' o 'prod')", target)
	}
	return plan, nil
}

// Apply ejecuta el deploy a dev via `make deploy-dev` desde la raiz del repo.
func Apply(target Target) (*Plan, error) {
	plan, err := Build(target)
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

	audit(root, map[string]any{
		"target":  string(plan.Target),
		"service": plan.Service,
		"ok":      runErr == nil,
	})

	if runErr != nil {
		return plan, fmt.Errorf("make deploy-dev fallo: %v", runErr)
	}
	return plan, nil
}

// Format produce el output humano-legible.
func Format(plan *Plan) string {
	var b strings.Builder
	fmt.Fprintln(&b, "=== DEPLOYER ===")
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
		fmt.Fprintln(&b, "\nsiguiente: deployer --apply")
		return b.String()
	}

	if plan.Output != "" {
		fmt.Fprintln(&b, "\n--- output make deploy-dev ---")
		out := plan.Output
		if len(out) > 4000 {
			out = "..." + out[len(out)-4000:]
		}
		fmt.Fprintln(&b, out)
	}
	fmt.Fprintln(&b, "\n✅ Deploy a dev completado.")
	return b.String()
}

// repoRoot localiza la raiz del repo: REMORA_ROOT > git rev-parse > cwd.
func repoRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv("REMORA_ROOT")); v != "" {
		return v, nil
	}
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		root := strings.TrimSpace(string(out))
		if root != "" {
			return root, nil
		}
	}
	return os.Getwd()
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// audit agrega una linea al log JSONL de operaciones aplicadas.
func audit(root string, fields map[string]any) {
	path := filepath.Join(root, "framework-deployer", "temp", "applied.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	entry := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"op":        "deploy",
		"fields":    fields,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}
