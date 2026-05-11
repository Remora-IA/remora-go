package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/alclessA0/remora-go/framework-deployer/internal/deployer"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		usage()
		return
	}

	r, err := deployer.NewRunbook()
	if err != nil {
		exitErr(err)
	}

	args := os.Args[1:]
	command := "plan"
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		command = args[0]
		args = args[1:]
	}

	envFlag := flagValue(args, "--env", "dev")
	if hasFlag(args, "--prod") {
		envFlag = "prod"
	}
	if hasFlag(args, "--dev") {
		envFlag = "dev"
	}
	env, err := deployer.ResolveEnv(envFlag, flagValue(args, "--intent", ""))
	if err != nil {
		exitErr(err)
	}
	ctx := context.Background()
	var out any

	switch command {
	case "plan":
		out, err = r.Plan(deployer.PlanInput{Intent: flagValue(args, "--intent", ""), RequestedEnv: envFlag})
	case "preflight":
		out = r.Preflight(ctx, env)
	case "test":
		out = r.Test(ctx, env)
	case "build":
		out = r.Build(ctx, env, flagValue(args, "--confirm", ""))
	case "watch-build":
		out = r.WatchBuild(ctx, flagValue(args, "--build-id", ""), intFlag(args, "--max-minutes", 15), intFlag(args, "--interval-seconds", 60))
	case "deploy":
		out = r.Deploy(ctx, env, flagValue(args, "--tag", ""), flagValue(args, "--confirm", ""))
	case "verify":
		out = r.Verify(ctx, env)
	case "verify-single-session":
		out = r.VerifySingleSession(ctx, env)
	case "verify-other-env-untouched":
		out = r.VerifyOtherEnvUntouched(ctx, env)
	case "diagnose":
		out = r.Diagnose(strings.Join(args, " "))
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		exitErr(err)
	}
	printJSON(out)
}

func usage() {
	fmt.Println(`Deployer CLI

USO:
  deployer plan --env dev --intent "deploy dev"
  deployer preflight --env dev
  deployer test --env dev
  deployer build --env dev
  deployer watch-build --build-id BUILD_ID --interval-seconds 60 --max-minutes 15
  deployer deploy --env dev --tag SHORTSHA-dev-YYYYMMDDHHMMSS
  deployer verify --env dev
  deployer verify-single-session --env dev
  deployer verify-other-env-untouched --env dev
  deployer diagnose "command not allowed: ./frameworkpaladin"

PROD:
  deployer plan --env prod
  deployer build --env prod --confirm "Confirmo deploy a PROD flujo-api"
  deployer deploy --env prod --tag TAG --confirm "Confirmo deploy a PROD flujo-api"

CONTRATO:
  - NO genera commits, tags ni push de git.
  - No usa make deploy ni deploy.sh.
  - DEV solo usa flujo-api-dev e imagen flujo-api-dev:<tag>.
  - PROD exige confirmacion textual exacta.
  - Todos los comandos imprimen JSON estructurado.
  - Lee REMORA_ROOT, PROJECT_ID, REGION, DEV_SERVICE de env vars (opcional).

Nota:
  watch-build hace polling; Cloud Build puede tardar varios minutos sin output local.`)
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		exitErr(err)
	}
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "deployer: %v\n", err)
	os.Exit(2)
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

func flagValue(args []string, name, def string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
		prefix := name + "="
		if strings.HasPrefix(a, prefix) {
			return strings.TrimPrefix(a, prefix)
		}
	}
	return def
}

func intFlag(args []string, name string, def int) int {
	v := flagValue(args, name, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
