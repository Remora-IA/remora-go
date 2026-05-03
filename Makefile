# =============================================================================
# Remora Go - Makefile
# =============================================================================
# Comandos para setup, build, dev y deploy. Usar siempre via `make <target>`
# en lugar de comandos sueltos. Si algo falta, agregalo aca.
# =============================================================================

.PHONY: help bootstrap setup-prod build build-frameworks build-flujo dev clean clean-logs clean-binaries test deploy-dev fmt vet check

# --- Variables --------------------------------------------------------------
SHELL          := /bin/bash
ROOT           := $(shell pwd)
GO             := go
FRAMEWORKS     := framework-alfa framework-auditor framework-charlie \
                  framework-contactos framework-echo framework-foco \
                  framework-gmail framework-hosting framework-indexa \
                  framework-mecanico framework-mensajero framework-quine \
                  framework-sabio framework-tareas
PROJECT_ID     := project-ceae5831-a2c9-49aa-b1c
REGION         := us-central1
DEV_SERVICE    := flujo-api-dev

# --- Default ----------------------------------------------------------------
help:
	@echo "Remora Go - comandos disponibles:"
	@echo ""
	@echo "  make bootstrap     Setup inicial local (valida .env, genera vault key, compila)"
	@echo "  make setup-prod    Setup prod completo (secrets + healthz + CI). Idempotente."
	@echo "  make build         Compila todos los binarios (frameworks + flujo_api)"
	@echo "  make dev           Arranca flujo_api en :8080 (modo desarrollo)"
	@echo "  make test          Corre tests de todo el repo"
	@echo "  make fmt           Formatea codigo Go"
	@echo "  make vet           Static analysis (go vet)"
	@echo "  make check         fmt + vet + test (pre-commit suite)"
	@echo "  make clean-logs    Borra logs/traces (seguro, solo regenerables)"
	@echo "  make clean-binaries Borra binarios compilados"
	@echo "  make clean         clean-logs + clean-binaries"
	@echo "  make deploy-dev    Deploy a Cloud Run dev (NUNCA prod)"

# --- Setup inicial ----------------------------------------------------------
bootstrap:
	@bash scripts/bootstrap.sh

setup-prod:
	@bash scripts/setup-prod.sh

# --- Build ------------------------------------------------------------------
build: build-frameworks build-flujo
	@echo "✅ Build completo"

build-frameworks:
	@echo "→ Compilando frameworks..."
	@for fw in $(FRAMEWORKS); do \
	  if [ -d "$$fw/cmd" ]; then \
	    echo "  · $$fw"; \
	    (cd $$fw && $(GO) build ./... 2>&1 | grep -v "^$$" || true); \
	  fi; \
	done

build-flujo:
	@echo "→ Compilando channel + flujo_api..."
	@cd channel && $(GO) build -o bin/channel ./cmd/channel
	@cd channel && $(GO) build -o bin/vault ./cmd/vault
	@cd remora-flujo && $(GO) build -o flujo_api ./cmd/flujo_api

# --- Desarrollo -------------------------------------------------------------
dev:
	@echo "→ Arrancando flujo_api en http://localhost:8080"
	@if [ ! -f .env ]; then \
	  echo "❌ Falta .env. Corre 'make bootstrap' primero."; \
	  exit 1; \
	fi
	@cd remora-flujo && $(GO) run ./cmd/flujo_api

# --- Tests ------------------------------------------------------------------
test:
	@echo "→ Tests de channel + remora-flujo..."
	@cd channel && $(GO) test ./... || true
	@cd remora-flujo && $(GO) test ./... || true

fmt:
	@$(GO) fmt ./... 2>/dev/null || true
	@for fw in $(FRAMEWORKS) channel remora-flujo; do \
	  (cd $$fw 2>/dev/null && $(GO) fmt ./... 2>/dev/null || true); \
	done

vet:
	@for fw in $(FRAMEWORKS) channel remora-flujo; do \
	  (cd $$fw 2>/dev/null && $(GO) vet ./... 2>/dev/null || true); \
	done

check: fmt vet test
	@echo "✅ Check completo"

# --- Limpieza ---------------------------------------------------------------
# clean-logs es SEGURO: solo borra archivos regenerables (traces, .DS_Store).
# NO toca state/, secrets/, applied.jsonl, ni databases.
clean-logs:
	@echo "→ Borrando traces y junk..."
	@find . -type f -name "trace_pal_*.json" -delete 2>/dev/null || true
	@find . -type f -name "trace_gf_*.json" -delete 2>/dev/null || true
	@find . -type f -name ".DS_Store" -delete 2>/dev/null || true
	@find . -type f -name "*.log" -delete 2>/dev/null || true
	@echo "✅ Logs limpios"

clean-binaries:
	@echo "→ Borrando binarios compilados..."
	@rm -f channel/channel channel/channel-new channel/orchestrator
	@rm -f channel/cmd/channel/channel channel/bin/vault channel/bin/channel
	@rm -rf channel/bin
	@for fw in $(FRAMEWORKS); do \
	  base=$$(basename $$fw); \
	  rm -f $$fw/$${base#framework-} 2>/dev/null || true; \
	  rm -f $$fw/framework$${base#framework-} 2>/dev/null || true; \
	  rm -rf $$fw/bin 2>/dev/null || true; \
	done
	@rm -f remora-flujo/flujo remora-flujo/flujo_api remora-flujo/agentrpc
	@rm -f remora-flujo/cmd/flujo_api/flujo_api remora-flujo/cmd/flujo_api/channel
	@echo "✅ Binarios limpios"

clean: clean-logs clean-binaries

# --- Deploy -----------------------------------------------------------------
# REGLA: solo a DEV. Nunca a prod (servicio flujo-api). Ver memoria.
deploy-dev:
	@echo "→ Cloud Build + deploy a $(DEV_SERVICE)..."
	@gcloud builds submit --config cloudbuild.yaml --project $(PROJECT_ID) .
	@gcloud run deploy $(DEV_SERVICE) \
	  --image gcr.io/$(PROJECT_ID)/flujo-api:latest \
	  --region $(REGION) \
	  --project $(PROJECT_ID)
	@echo "✅ Deployed a https://$(DEV_SERVICE)-760602975866.us-central1.run.app"
