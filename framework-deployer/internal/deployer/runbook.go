package deployer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Runbook struct {
	Config Config
	Root   string
	Runner Runner
	Now    func() time.Time
}

type PlanInput struct {
	Intent       string `json:"intent,omitempty"`
	RequestedEnv string `json:"requested_env,omitempty"`
}

type PlanResult struct {
	Env                  Env      `json:"env"`
	Service              string   `json:"service"`
	URL                  string   `json:"url"`
	Project              string   `json:"project"`
	Region               string   `json:"region"`
	Image                string   `json:"image"`
	RequiresConfirmation bool     `json:"requires_confirmation"`
	Warning              string   `json:"warning,omitempty"`
	ForbiddenServices    []string `json:"forbidden_services,omitempty"`
	ForbiddenImages      []string `json:"forbidden_images,omitempty"`
	ConfirmationPhrase   string   `json:"confirmation_phrase,omitempty"`
}

type Report struct {
	Command   string          `json:"command"`
	Env       Env             `json:"env,omitempty"`
	Project   string          `json:"project,omitempty"`
	Region    string          `json:"region,omitempty"`
	Service   string          `json:"service,omitempty"`
	URL       string          `json:"url,omitempty"`
	Image     string          `json:"image,omitempty"`
	Tag       string          `json:"tag,omitempty"`
	BuildID   string          `json:"build_id,omitempty"`
	Revision  string          `json:"revision,omitempty"`
	Status    string          `json:"status"`
	Steps     []CommandResult `json:"steps,omitempty"`
	Plan      *PlanResult     `json:"plan,omitempty"`
	Diagnosis *Diagnosis      `json:"diagnosis,omitempty"`
	Next      []string        `json:"next,omitempty"`
}

func NewRunbook() (*Runbook, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	return &Runbook{Config: DefaultConfig(), Root: root, Runner: RealRunner{}, Now: time.Now}, nil
}

func (r *Runbook) Plan(input PlanInput) (PlanResult, error) {
	env, err := ResolveEnv(input.RequestedEnv, input.Intent)
	if err != nil {
		return PlanResult{}, err
	}
	target, err := r.Config.Target(env)
	if err != nil {
		return PlanResult{}, err
	}
	other, _ := r.Config.Target(OtherEnv(env))
	result := PlanResult{
		Env:                  env,
		Service:              target.Service,
		URL:                  target.URL,
		Project:              r.Config.Project,
		Region:               r.Config.Region,
		Image:                target.Image,
		RequiresConfirmation: target.RequiresConfirmation,
		ForbiddenServices:    []string{other.Service},
		ForbiddenImages:      []string{other.Image + ":<tag>"},
	}
	if env == Prod {
		result.Warning = "PROD deploy requested. Require explicit user confirmation."
		result.ConfirmationPhrase = ProdConfirmationPhrase
	}
	return result, nil
}

func (r *Runbook) Preflight(ctx context.Context, env Env) Report {
	rep := r.baseReport("preflight", env)
	target, err := r.Config.Target(env)
	if err != nil {
		return failedTarget(rep, r.Root, err)
	}
	plan := r.mustPlan(env)
	rep.Plan = &plan
	rep.Steps = append(rep.Steps,
		r.namedRun(ctx, "git_status", r.Root, "git", "status", "--short"),
		r.namedRun(ctx, "git_commit", r.Root, "git", "rev-parse", "--short", "HEAD"),
		fileCheck(r.Root, "cloudbuild_yaml", "cloudbuild.yaml"),
		fileCheck(r.Root, "api_rest_dockerfile", "remora-flujo/cmd/api_rest/Dockerfile"),
		r.namedRun(ctx, "gcloud_account", r.Root, "gcloud", "config", "get-value", "account"),
		r.namedRun(ctx, "gcloud_project", r.Root, "gcloud", "config", "get-value", "project"),
		r.namedRun(ctx, "target_service", r.Root, "gcloud", "run", "services", "describe", target.Service, "--project="+r.Config.Project, "--region="+r.Config.Region, "--format=value(metadata.name)"),
	)
	other, _ := r.Config.Target(OtherEnv(env))
	rep.Steps = append(rep.Steps, r.namedRun(ctx, "protected_other_service", r.Root, "gcloud", "run", "services", "describe", other.Service, "--project="+r.Config.Project, "--region="+r.Config.Region, "--format=value(status.latestReadyRevisionName,spec.template.spec.containers[0].image)"))
	if env == Dev {
		rep.Steps = append(rep.Steps, guardStep("dev_guardrails", r.Root, []string{"deploy.sh", "flujo-api", "gcr.io/" + r.Config.Project + "/flujo-api:<tag>"}, []string{target.Service, target.Image + ":<tag>"}))
	}
	rep.Status = aggregateStatus(rep.Steps)
	return rep
}

func (r *Runbook) Test(ctx context.Context, env Env) Report {
	rep := r.baseReport("test", env)
	rep.Steps = append(rep.Steps,
		r.namedRun(ctx, "api_rest_tests", filepath.Join(r.Root, "remora-flujo"), "go", "test", "./cmd/api_rest"),
		r.namedRun(ctx, "channel_deploy_tests", filepath.Join(r.Root, "channel"), "go", "test", "./adapter", "./cmd/channel", "./cmd/vault", "./internal", "./manifest", "./vault"),
		r.namedRun(ctx, "paladin_tests", filepath.Join(r.Root, "framework-paladin"), "go", "test", "./paladin"),
		r.namedShell(ctx, "paladin_lint", filepath.Join(r.Root, "framework-paladin"), "go run ./cmd/paladin lint .. 2>/dev/null | grep '\\[fail\\]' || true"),
	)
	last := len(rep.Steps) - 1
	if last >= 0 && strings.TrimSpace(rep.Steps[last].Output) != "" {
		rep.Steps[last].OK = false
		rep.Steps[last].Error = "paladin lint imprimió fallos"
	}
	rep.Status = aggregateStatus(rep.Steps)
	return rep
}

func (r *Runbook) Build(ctx context.Context, env Env, confirmation string) Report {
	rep := r.baseReport("build", env)
	target, err := r.Config.Target(env)
	if err != nil {
		return failedTarget(rep, r.Root, err)
	}
	if blocked := prodBlock(env, confirmation); blocked != "" {
		rep.Status = "blocked"
		rep.Steps = append(rep.Steps, skipped("prod_confirmation", r.Root, "", blocked))
		return rep
	}
	short := r.namedRun(ctx, "git_commit", r.Root, "git", "rev-parse", "--short", "HEAD")
	rep.Steps = append(rep.Steps, short)
	if !short.OK {
		rep.Status = "failed"
		return rep
	}
	tag := strings.TrimSpace(short.Output) + "-" + string(env) + "-" + r.Now().Format("20060102150405")
	rep.Tag = tag
	rep.Image = target.Image + ":" + tag
	subs := fmt.Sprintf("_PROJECT_ID=%s,_SERVICE_NAME=%s,_REGION=%s,_SHORT_SHA=%s", r.Config.Project, target.Service, r.Config.Region, tag)
	build := r.namedRun(ctx, "cloud_build_submit", r.Root, "gcloud", "builds", "submit", "--async", "--config=cloudbuild.yaml", "--project="+r.Config.Project, "--substitutions="+subs, ".")
	rep.Steps = append(rep.Steps, build)
	rep.BuildID = extractBuildID(build.Output)
	if rep.BuildID == "" && build.OK {
		list := r.namedRun(ctx, "cloud_build_find", r.Root, "gcloud", "builds", "list", "--project="+r.Config.Project, "--limit=5", "--format=value(id,status,substitutions._SERVICE_NAME,substitutions._SHORT_SHA)")
		rep.Steps = append(rep.Steps, list)
		rep.BuildID = findBuildID(list.Output, target.Service, tag)
	}
	rep.Status = aggregateStatus(rep.Steps)
	if rep.Status == "ok" && rep.BuildID == "" {
		rep.Status = "needs_attention"
		rep.Next = append(rep.Next, "Ejecutar watch-build; gcloud no imprimió build ID identificable.")
	}
	return rep
}

func (r *Runbook) WatchBuild(ctx context.Context, buildID string, maxMinutes, intervalSeconds int) Report {
	rep := Report{Command: "watch-build", Project: r.Config.Project, Region: r.Config.Region, BuildID: buildID, Status: "working"}
	if maxMinutes <= 0 {
		maxMinutes = 15
	}
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}
	deadline := r.Now().Add(time.Duration(maxMinutes) * time.Minute)
	for {
		if strings.TrimSpace(buildID) == "" {
			step := r.namedRun(ctx, "cloud_builds_list", r.Root, "gcloud", "builds", "list", "--project="+r.Config.Project, "--limit=5", "--format=table(id,status,createTime,substitutions._SERVICE_NAME,substitutions._SHORT_SHA)")
			rep.Steps = append(rep.Steps, step)
			rep.Status = aggregateStatus(rep.Steps)
			return rep
		}
		describe := r.namedRun(ctx, "cloud_build_describe", r.Root, "gcloud", "builds", "describe", buildID, "--project="+r.Config.Project, "--format=value(status,logUrl)")
		rep.Steps = append(rep.Steps, describe)
		status := firstField(describe.Output)
		switch status {
		case "SUCCESS":
			rep.Status = "success"
			return rep
		case "FAILURE", "CANCELLED", "TIMEOUT", "EXPIRED":
			rep.Steps = append(rep.Steps, r.buildLogTail(ctx, buildID))
			rep.Status = "failed"
			return rep
		}
		rep.Steps = append(rep.Steps, r.buildLogTail(ctx, buildID))
		if !r.Now().Before(deadline) {
			rep.Status = "human_timeout"
			rep.Next = append(rep.Next, "Cloud Build no llegó a estado terminal. Revisar logUrl y consola; no asumir fallo si sigue WORKING.")
			return rep
		}
		timer := time.NewTimer(time.Duration(intervalSeconds) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			rep.Status = "cancelled"
			return rep
		case <-timer.C:
		}
	}
}

func (r *Runbook) Deploy(ctx context.Context, env Env, tag, confirmation string) Report {
	rep := r.baseReport("deploy", env)
	target, err := r.Config.Target(env)
	if err != nil {
		return failedTarget(rep, r.Root, err)
	}
	if blocked := prodBlock(env, confirmation); blocked != "" {
		rep.Status = "blocked"
		rep.Steps = append(rep.Steps, skipped("prod_confirmation", r.Root, "", blocked))
		return rep
	}
	if strings.TrimSpace(tag) == "" {
		rep.Status = "blocked"
		rep.Steps = append(rep.Steps, skipped("tag_required", r.Root, "", "deploy requiere --tag exacto; no se usa latest"))
		return rep
	}
	image := target.Image + ":" + tag
	args := []string{"run", "deploy", target.Service, "--project=" + r.Config.Project, "--region=" + r.Config.Region, "--platform=managed", "--image=" + image, "--allow-unauthenticated", "--memory=1Gi", "--cpu=2", "--timeout=300s", "--max-instances=10", "--format=value(status.latestReadyRevisionName)"}
	if target.DevMode {
		args = append(args, "--update-env-vars=REMORA_DEV_MODE=true")
	}
	step := r.namedRun(ctx, "cloud_run_deploy", r.Root, "gcloud", args...)
	rep.Steps = append(rep.Steps, step)
	rep.Image = image
	rep.Tag = tag
	rep.Revision = strings.TrimSpace(step.Output)
	rep.Steps = append(rep.Steps, r.namedRun(ctx, "cloud_run_traffic", r.Root, "gcloud", "run", "services", "describe", target.Service, "--project="+r.Config.Project, "--region="+r.Config.Region, "--format=value(status.traffic[0].percent,status.traffic[0].revisionName)"))
	rep.Status = aggregateStatus(rep.Steps)
	return rep
}

func (r *Runbook) Verify(ctx context.Context, env Env) Report {
	rep := r.baseReport("verify", env)
	target, err := r.Config.Target(env)
	if err != nil {
		return failedTarget(rep, r.Root, err)
	}
	base := strings.TrimRight(target.URL, "/")
	rep.Steps = append(rep.Steps,
		r.namedShell(ctx, "health", r.Root, fmt.Sprintf("curl -fsS %q", base+"/health")),
		r.namedShell(ctx, "frontend", r.Root, fmt.Sprintf("curl -fsS %q | head -n 20", base+"/")),
		r.namedShell(ctx, "frameworks_testable", r.Root, fmt.Sprintf("curl -fsS %q | python3 -c 'import json,sys; d=json.load(sys.stdin)[\"data\"]; print(len(d), sorted(x[\"name\"] for x in d))'", base+"/api/v1/frameworks/testable")),
		r.namedShell(ctx, "frameworks_chainable", r.Root, fmt.Sprintf("curl -fsS %q | python3 -c 'import json,sys; d=json.load(sys.stdin)[\"data\"]; print(len(d), sorted(x[\"name\"] for x in d))'", base+"/api/v1/frameworks/chainable")),
	)
	rep.Status = aggregateStatus(rep.Steps)
	return rep
}

func (r *Runbook) VerifySingleSession(ctx context.Context, env Env) Report {
	rep := r.baseReport("verify-single-session", env)
	target, err := r.Config.Target(env)
	if err != nil {
		return failedTarget(rep, r.Root, err)
	}
	base := strings.TrimRight(target.URL, "/")
	echo := fmt.Sprintf("BASE=%q\nCONV=$(curl -fsS -X POST \"$BASE/api/v1/conversations-single\" -H 'Content-Type: application/json' -d '{\"title\":\"test echo deploy\",\"framework\":\"echo\"}' | python3 -c 'import json,sys; print(json.load(sys.stdin)[\"data\"][\"id\"])')\ncurl -fsS -X POST \"$BASE/api/v1/conversations-single/$CONV/messages\" -H 'Content-Type: application/json' -d '{\"content\":\"Hola, quiero probar echo en dev\"}' | python3 -c 'import json,sys; d=json.load(sys.stdin)[\"data\"]; print(\"idle=\", d.get(\"idle\")); print(\"framework_message=\", bool(d.get(\"framework_message\"))); print(\"status=\", (d.get(\"framework_message\") or {}).get(\"status\"))'", base)
	paladin := fmt.Sprintf("BASE=%q\nCONV=$(curl -fsS -X POST \"$BASE/api/v1/conversations-single\" -H 'Content-Type: application/json' -d '{\"title\":\"test paladin deploy\",\"framework\":\"paladin\"}' | python3 -c 'import json,sys; print(json.load(sys.stdin)[\"data\"][\"id\"])')\ncurl -fsS -X POST \"$BASE/api/v1/conversations-single/$CONV/messages\" -H 'Content-Type: application/json' -d '{\"content\":\"/audit\"}' | python3 -c 'import json,sys; d=json.load(sys.stdin)[\"data\"]; s=json.dumps(d); print(s[:1200]); assert \"command not allowed\" not in s.lower()'", base)
	rep.Steps = append(rep.Steps, r.namedShell(ctx, "echo_single_session", r.Root, echo), r.namedShell(ctx, "paladin_single_session", r.Root, paladin))
	rep.Status = aggregateStatus(rep.Steps)
	return rep
}

func (r *Runbook) VerifyOtherEnvUntouched(ctx context.Context, env Env) Report {
	other := OtherEnv(env)
	rep := r.baseReport("verify-other-env-untouched", other)
	target, err := r.Config.Target(other)
	if err != nil {
		return failedTarget(rep, r.Root, err)
	}
	rep.Steps = append(rep.Steps, r.namedRun(ctx, "other_env_revision_image", r.Root, "gcloud", "run", "services", "describe", target.Service, "--project="+r.Config.Project, "--region="+r.Config.Region, "--format=value(status.latestReadyRevisionName,spec.template.spec.containers[0].image)"))
	rep.Status = aggregateStatus(rep.Steps)
	return rep
}

func (r *Runbook) Diagnose(text string) Report {
	d := DiagnoseText(text)
	return Report{Command: "diagnose", Status: "ok", Diagnosis: &d, Next: d.Actions}
}

func (r *Runbook) baseReport(command string, env Env) Report {
	target, _ := r.Config.Target(env)
	return Report{Command: command, Env: env, Project: r.Config.Project, Region: r.Config.Region, Service: target.Service, URL: target.URL, Image: target.Image, Status: "ok"}
}

func (r *Runbook) mustPlan(env Env) PlanResult {
	p, _ := r.Plan(PlanInput{RequestedEnv: string(env)})
	return p
}

func (r *Runbook) namedRun(ctx context.Context, name, cwd, cmd string, args ...string) CommandResult {
	res := r.Runner.Run(ctx, cwd, cmd, args...)
	res.Name = name
	return res
}

func (r *Runbook) namedShell(ctx context.Context, name, cwd, script string) CommandResult {
	res := r.Runner.RunShell(ctx, cwd, script)
	res.Name = name
	return res
}

func (r *Runbook) buildLogTail(ctx context.Context, buildID string) CommandResult {
	return r.namedShell(ctx, "cloud_build_logs_tail", r.Root, fmt.Sprintf("gcloud builds log %s --project=%s --stream --format='value(textPayload)' | tail -n 120", buildID, r.Config.Project))
}

func fileCheck(root, name, rel string) CommandResult {
	path := filepath.Join(root, rel)
	_, err := os.Stat(path)
	return CommandResult{Name: name, Cwd: root, Command: "test -f " + rel, OK: err == nil, ExitCode: boolExit(err == nil), Error: errString(err)}
}

func guardStep(name, root string, forbidden, allowed []string) CommandResult {
	data, _ := json.Marshal(map[string][]string{"forbidden": forbidden, "allowed": allowed})
	return CommandResult{Name: name, Cwd: root, Command: "guardrails", OK: true, Output: string(data)}
}

func failedTarget(rep Report, root string, err error) Report {
	rep.Status = "failed"
	rep.Steps = append(rep.Steps, skipped("target", root, "", err.Error()))
	return rep
}

func boolExit(ok bool) int {
	if ok {
		return 0
	}
	return 1
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func aggregateStatus(steps []CommandResult) string {
	for _, s := range steps {
		if !s.OK {
			if s.Skipped {
				return "blocked"
			}
			return "failed"
		}
	}
	return "ok"
}

func prodBlock(env Env, confirmation string) string {
	if env != Prod {
		return ""
	}
	if strings.TrimSpace(confirmation) == ProdConfirmationPhrase {
		return ""
	}
	return "PROD requiere confirmación exacta: " + ProdConfirmationPhrase
}

var buildIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^ID:\s*([a-f0-9-]+)\s*$`),
	regexp.MustCompile(`(?m)builds/([a-f0-9-]+)`),
}

func extractBuildID(output string) string {
	for _, re := range buildIDPatterns {
		if m := re.FindStringSubmatch(output); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func findBuildID(output, service, tag string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, service) && strings.Contains(line, tag) {
			return firstField(line)
		}
	}
	return ""
}

func firstField(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
