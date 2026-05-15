package main

import (
	"fmt"
	"os"

	"github.com/alclessA0/remora-go/framework-charlie/internal/charlie"
)

func main() {
	args, root, err := stripGlobalRoot(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", err)
		os.Exit(2)
	}
	if err := charlie.SetRepoRoot(root); err != nil {
		fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", err)
		os.Exit(1)
	}

	command := "status"
	if len(args) > 0 {
		command = args[0]
	}
	rest := []string{}
	if len(args) > 1 {
		rest = args[1:]
	}

	switch command {
	case "doctor":
		apply := len(rest) > 0 && rest[0] == "--apply"
		var report *charlie.DoctorReport
		var reportErr error
		if apply {
			report, reportErr = charlie.ApplyDoctor()
		} else {
			report, reportErr = charlie.RunDoctor()
		}
		if reportErr != nil {
			fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", reportErr)
			os.Exit(1)
		}
		fmt.Println(charlie.FormatDoctorReport(report))
		if report.OverallHealth == charlie.SeverityBlocker || report.OverallHealth == charlie.SeverityCritical {
			os.Exit(2)
		}
		return
	case "apply-propose":
		apply := false
		push := false
		for _, a := range rest {
			if a == "--apply" {
				apply = true
			}
			if a == "--push" {
				push = true
			}
		}
		var plan *charlie.ApplyProposePlan
		var err error
		if apply {
			plan, err = charlie.ApplyApplyPropose(push)
		} else {
			plan, err = charlie.BuildApplyProposePlan()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", err)
			os.Exit(1)
		}
		fmt.Println(charlie.FormatApplyProposePlan(plan))
		if len(plan.Blockers) > 0 {
			os.Exit(2)
		}
		return
	case "plan":
		intent := ""
		for i := 0; i < len(rest); i++ {
			if rest[i] == "--intent" && i+1 < len(rest) {
				intent = rest[i+1]
				break
			}
		}
		if intent == "" {
			fmt.Fprintln(os.Stderr, "uso: charlie plan --intent \"descripcion del objetivo\"")
			os.Exit(2)
		}
		plan := charlie.BuildIntentPlan(intent)
		fmt.Println(charlie.FormatIntentPlan(plan))
		return
	case "backup":
		path, err := charlie.BackupWorkingTree()
		if err != nil {
			fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Backup creado: %s\n", path)
		return
	case "preflight":
		report, err := charlie.Preflight()
		if err != nil {
			fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", err)
			os.Exit(1)
		}
		fmt.Println(charlie.FormatPreflight(report))
		if len(report.Blockers) > 0 {
			os.Exit(2)
		}
		return
	case "amend-plan":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "uso: charlie amend-plan vVERSION")
			os.Exit(2)
		}
		plan, err := charlie.BuildAmendPlan(rest[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", err)
			os.Exit(1)
		}
		fmt.Println(charlie.FormatAmendPlan(plan))
		if len(plan.Blockers) > 0 {
			os.Exit(2)
		}
		return
	case "reconcile-draft":
		plan, err := charlie.BuildReconcileDraftPlan()
		if err != nil {
			fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", err)
			os.Exit(1)
		}
		fmt.Println(charlie.FormatReconcilePlan(plan))
		if len(plan.Blockers) > 0 {
			os.Exit(2)
		}
		return
	case "repair-release":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "uso: charlie repair-release vVERSION [--apply]")
			os.Exit(2)
		}
		apply := len(rest) > 1 && rest[1] == "--apply"
		var plan *charlie.RepairReleasePlan
		var err error
		if apply {
			plan, err = charlie.ApplyRepairRelease(rest[0])
		} else {
			plan, err = charlie.BuildRepairReleasePlan(rest[0])
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", err)
			os.Exit(1)
		}
		fmt.Println(charlie.FormatRepairReleasePlan(plan))
		if len(plan.Blockers) > 0 {
			os.Exit(2)
		}
		return
	case "publish-draft":
		apply := len(rest) > 0 && rest[0] == "--apply"
		var plan *charlie.PublishDraftPlan
		var err error
		if apply {
			plan, err = charlie.ApplyPublishDraft()
		} else {
			plan, err = charlie.BuildPublishDraftPlan()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", err)
			os.Exit(1)
		}
		fmt.Println(charlie.FormatPublishDraftPlan(plan))
		if len(plan.Blockers) > 0 {
			os.Exit(2)
		}
		return
	case "publish-tag":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "uso: charlie publish-tag vVERSION [--apply]")
			os.Exit(2)
		}
		apply := len(rest) > 1 && rest[1] == "--apply"
		var plan *charlie.PublishTagPlan
		var err error
		if apply {
			plan, err = charlie.ApplyPublishTag(rest[0])
		} else {
			plan, err = charlie.BuildPublishTagPlan(rest[0])
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", err)
			os.Exit(1)
		}
		fmt.Println(charlie.FormatPublishTagPlan(plan))
		if len(plan.Blockers) > 0 {
			os.Exit(2)
		}
		return
	case "clean-traces":
		apply := false
		root := charlie.CurrentRepoRoot()
		for i := 0; i < len(rest); i++ {
			if rest[i] == "--apply" {
				apply = true
			}
		}
		res, err := charlie.CleanTraces(root, apply)
		if err != nil {
			fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", err)
			os.Exit(1)
		}
		fmt.Println(charlie.FormatCleanTraces(res))
		return
	case "publish-main":
		apply := len(rest) > 0 && rest[0] == "--apply"
		var plan *charlie.PublishMainPlan
		var err error
		if apply {
			plan, err = charlie.ApplyPublishMain()
		} else {
			plan, err = charlie.BuildPublishMainPlan()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", err)
			os.Exit(1)
		}
		fmt.Println(charlie.FormatPublishMainPlan(plan))
		if len(plan.Blockers) > 0 {
			os.Exit(2)
		}
		return
	}

	report, err := charlie.BuildReport()
	if err != nil {
		fmt.Fprintf(os.Stderr, "=== CHARLIE (ERROR) ===\n\n%v\n", err)
		os.Exit(1)
	}

	switch command {
	case "status":
		fmt.Println(charlie.FormatStatus(report))
	case "changelog":
		if len(report.Changes) == 0 {
			fmt.Println("✅ Repo limpio, no hay cambios pendientes")
			return
		}
		fmt.Println(report.Changelog)
	case "propose":
		fmt.Println(charlie.FormatProposal(report))
	case "validate":
		fmt.Println(charlie.FormatValidation(report))
		if len(charlie.ValidateReport(report)) > 0 {
			os.Exit(1)
		}
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n\n", command)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`Charlie CLI

USO:
  charlie [--root PATH] <comando>
  charlie status          Muestra estado, tag actual y siguiente version lineal
  charlie doctor [--apply] (v0.1.8) diagnostica integridad del repo; --apply
                  ejecuta recetas seguras (fetch-missing-objects, disable-gc-auto, etc.)
  charlie plan --intent "..." (v0.1.8) devuelve la secuencia de comandos Charlie
                  que satisface un objetivo en lenguaje natural
  charlie backup      Crea backup liviano del working tree fuera del repo
  charlie preflight   Crea backup y bloquea versionado inseguro (incluye doctor)
  charlie changelog   Genera CHANGELOG detallado por archivo desde git diff
  charlie propose     Genera changelog obligatorio y un unico commit propuesto
  charlie apply-propose [--apply] [--push] (v0.1.8)
                  Cierra el happy path: escribe CHANGELOG, stagea, commitea,
                  taggea y (opcional) pushea, todo vuelto por runGitControlled
                  y auditado en framework-charlie/temp/applied.jsonl
  charlie validate    Valida version, changelog y formato de commit
  charlie amend-plan vVERSION
                  Diagnostica como agregar cambios a una release existente
  charlie reconcile-draft
                  Diagnostica divergencias entre draft y su upstream
  charlie repair-release vVERSION [--apply]
                  Repara una release existente desde la base canonica
  charlie publish-draft [--apply]
                  Publica draft con estrategia segura
  charlie publish-tag vVERSION [--apply]
                  Publica o actualiza el tag remoto con lease
  charlie publish-main [--apply]
                  Actualiza main para que sea copia exacta de draft
  charlie clean-traces [--apply] [--root PATH] (v0.1.11)
                  Lista (o borra con --apply) archivos regenerables seguros:
                  trace_pal_*.json, trace_gf_*.json, .DS_Store. NUNCA toca
                  state, secrets, applied.jsonl, sessions ni databases.
  Si no pasas --root, Charlie opera sobre el repo git actual. Si el cwd no
                  esta dentro de un repo, intenta usar el repo que contiene
                  framework-charlie.

CONTRATO:
  Charlie no ejecuta git manual. Los comandos del framework pueden aplicar
  operaciones Git controladas con backup y validaciones.
  El commit siempre usa: chore: commit vVERSION - descripcion principal`)
}

func stripGlobalRoot(args []string) ([]string, string, error) {
	filtered := make([]string, 0, len(args))
	root := ""
	for i := 0; i < len(args); i++ {
		if args[i] != "--root" {
			filtered = append(filtered, args[i])
			continue
		}
		if i+1 >= len(args) {
			return nil, "", fmt.Errorf("uso: --root PATH")
		}
		root = args[i+1]
		i++
	}
	return filtered, root, nil
}
