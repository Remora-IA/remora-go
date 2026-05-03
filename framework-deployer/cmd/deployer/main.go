package main

import (
	"fmt"
	"os"

	"github.com/alclessA0/remora-go/framework-deployer/internal/deployer"
)

func main() {
	apply := false
	target := deployer.Dev

	for _, a := range os.Args[1:] {
		switch a {
		case "--apply":
			apply = true
		case "--prod":
			target = deployer.Prod
		case "--dev":
			target = deployer.Dev
		case "-h", "--help":
			usage()
			return
		}
	}

	var plan *deployer.Plan
	var err error
	if apply {
		plan, err = deployer.Apply(target)
	} else {
		plan, err = deployer.Build(target)
	}
	if plan != nil {
		fmt.Println(deployer.Format(plan))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "=== DEPLOYER (ERROR) ===\n\n%v\n", err)
		os.Exit(2)
	}
	if plan != nil && len(plan.Blockers) > 0 {
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`Deployer CLI

USO:
  deployer                     Plan: muestra que se haria (default: dev)
  deployer --apply             Deploya a dev (flujo-api-dev)
  deployer --prod              SIEMPRE bloqueado. Prod solo se actualiza a mano.
  deployer --apply --dev       Mismo que --apply (explicito)

CONTRATO:
  - NO genera commits, tags ni push de git.
  - NO toca produccion bajo ninguna circunstancia.
  - Audita cada deploy en framework-deployer/temp/applied.jsonl.
  - Lee REMORA_ROOT, PROJECT_ID, REGION, DEV_SERVICE de env vars (opcional).`)
}
