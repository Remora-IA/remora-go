package deployer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Env string

const (
	Dev  Env = "dev"
	Prod Env = "prod"
)

const ProdConfirmationPhrase = "Confirmo deploy a PROD flujo-api"

type Config struct {
	Project string             `json:"project"`
	Region  string             `json:"region"`
	Targets map[Env]TargetSpec `json:"targets"`
}

type TargetSpec struct {
	Service              string `json:"service"`
	URL                  string `json:"url"`
	Image                string `json:"image"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	DevMode              bool   `json:"dev_mode"`
}

func DefaultConfig() Config {
	project := envOr("PROJECT_ID", "project-ceae5831-a2c9-49aa-b1c")
	region := envOr("REGION", "us-central1")
	return Config{
		Project: project,
		Region:  region,
		Targets: map[Env]TargetSpec{
			Dev: {
				Service:              envOr("DEV_SERVICE", "flujo-api-dev"),
				URL:                  "https://flujo-api-dev-760602975866.us-central1.run.app/",
				Image:                fmt.Sprintf("gcr.io/%s/flujo-api-dev", project),
				RequiresConfirmation: false,
				DevMode:              true,
			},
			Prod: {
				Service:              "flujo-api",
				URL:                  "https://flujo-api-760602975866.us-central1.run.app/",
				Image:                fmt.Sprintf("gcr.io/%s/flujo-api", project),
				RequiresConfirmation: true,
				DevMode:              false,
			},
		},
	}
}

func ResolveEnv(requestedEnv, intent string) (Env, error) {
	v := strings.ToLower(strings.TrimSpace(requestedEnv))
	switch v {
	case "", "auto":
	case "dev", "develop", "development", "desarrollo":
		return Dev, nil
	case "prod", "production", "produccion", "producción":
		return Prod, nil
	default:
		return "", fmt.Errorf("ambiente desconocido: %q", requestedEnv)
	}
	text := strings.ToLower(intent)
	if strings.Contains(text, "prod") || strings.Contains(text, "produccion") || strings.Contains(text, "producción") {
		return Prod, nil
	}
	return Dev, nil
}

func (c Config) Target(env Env) (TargetSpec, error) {
	t, ok := c.Targets[env]
	if !ok {
		return TargetSpec{}, fmt.Errorf("target no configurado: %q", env)
	}
	return t, nil
}

func OtherEnv(env Env) Env {
	if env == Prod {
		return Dev
	}
	return Prod
}

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
