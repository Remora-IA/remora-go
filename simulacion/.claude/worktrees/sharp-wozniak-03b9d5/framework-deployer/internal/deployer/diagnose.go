package deployer

import "strings"

type Diagnosis struct {
	Diagnostic  string   `json:"diagnostic"`
	Remediation string   `json:"remediation"`
	Actions     []string `json:"actions,omitempty"`
}

func DiagnoseText(text string) Diagnosis {
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "no new output since last status check"):
		return Diagnosis{
			Diagnostic:  "Cloud Build puede seguir trabajando aunque la terminal no imprima salida.",
			Remediation: "No declarar fallo. Usar watch-build, builds list, builds describe y logs; esperar 10-15 minutos antes de timeout humano.",
			Actions: []string{
				"gcloud builds list --project=project-ceae5831-a2c9-49aa-b1c --limit=5 --format='table(id,status,createTime,substitutions._SERVICE_NAME,substitutions._SHORT_SHA)'",
				"gcloud builds describe <BUILD_ID> --project=project-ceae5831-a2c9-49aa-b1c --format='value(status,logUrl)'",
				"gcloud builds log <BUILD_ID> --project=project-ceae5831-a2c9-49aa-b1c --stream --format='value(textPayload)' | tail -n 120",
			},
		}
	case strings.Contains(t, "/healthz") && strings.Contains(t, "404"):
		return Diagnosis{Diagnostic: "/healthz no es el endpoint correcto para Remora Flujo.", Remediation: "Verificar con /health.", Actions: []string{"curl -fsS \"$BASE/health\""}}
	case strings.Contains(t, "sslcertverificationerror"):
		return Diagnosis{Diagnostic: "Fallo local de certificados SSL en Python, no necesariamente fallo de API.", Remediation: "Usar curl para descargar la respuesta y pipe a python3 para parsear JSON.", Actions: []string{"curl -fsS URL | python3 -c 'import json,sys; print(json.load(sys.stdin))'"}}
	case strings.Contains(t, "command not allowed") && strings.Contains(t, "frameworkpaladin"):
		return Diagnosis{Diagnostic: "Channel probablemente bloquea el binario de Paladin por whitelist.", Remediation: "Verificar channel/internal/whitelist.go y asegurar './frameworkpaladin': true.", Actions: []string{"grep -n \"frameworkpaladin\" channel/internal/whitelist.go"}}
	case strings.Contains(t, "fmt.println arg list ends with redundant newline"):
		return Diagnosis{Diagnostic: "Fallo preexistente de go vet en channel/cmd/orchestrator.", Remediation: "No bloquear deploy si pasan los paquetes usados por Cloud Run: ./adapter ./cmd/channel ./cmd/vault ./internal ./manifest ./vault."}
	case strings.Contains(t, "command not found") && strings.Contains(t, "gcloud"):
		return Diagnosis{Diagnostic: "gcloud no está instalado o no está en PATH.", Remediation: "Instalar/configurar Google Cloud SDK antes de ejecutar preflight/build/deploy."}
	case strings.Contains(t, "manifest uses go run") || strings.Contains(t, "runtime no tiene go"):
		return Diagnosis{Diagnostic: "El framework puede estar intentando usar go run en runtime.", Remediation: "Verificar cloudbuild.yaml, Dockerfile, chmod, whitelist de Channel, single_wrapper.go y drivers.go."}
	default:
		return Diagnosis{Diagnostic: "No hay diagnóstico conocido para esta salida.", Remediation: "Revisar el comando fallido, últimos logs y confirmar que target, imagen y servicio corresponden al ambiente solicitado."}
	}
}
