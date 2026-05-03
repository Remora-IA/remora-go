# Cloud Run deployment for flujo_api + Channel + Frameworks
# Opción A: binaries pre-compilados
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ENV FLUJO_API_PORT=8080
ENV PORT=8080
ENV CHANNEL_PORT=8765
ENV CHANNEL_URL=http://localhost:8765
ENV CHANNEL_BASE_DIR=/workspace

ENV CHANNEL_API_KEY=${CHANNEL_API_KEY:-test-key-001}
ENV CHANNEL_API_KEYS=${CHANNEL_API_KEYS:-${CHANNEL_API_KEY}}
ENV MINIMAX_API_KEY=${MINIMAX_API_KEY:-}
ENV GROQ_API_KEY=${GROQ_API_KEY:-}
ENV GEMINI_API_KEY=${GEMINI_API_KEY:-}
ENV SMTP_HOST=${SMTP_HOST:-mail.patriciastocker.com}
ENV SMTP_PORT=${SMTP_PORT:-587}
ENV SMTP_USER=${SMTP_USER:-}
ENV SMTP_PASS=${SMTP_PASS:-}
ENV SMTP_FROM=${SMTP_FROM:-}
ENV TEST_EMAIL_RECIPIENT=${TEST_EMAIL_RECIPIENT:-tom3bs@gmail.com}
ENV REMORA_VAULT_KEY=${REMORA_VAULT_KEY:-}
ENV REMORA_VAULT_DIR=/workspace/channel/vault_data
ENV REMORA_ROOT=/workspace

WORKDIR /

COPY remora-flujo/cmd/flujo_api/flujo_api /flujo_api
COPY remora-flujo/cmd/flujo_api/channel /channel
COPY remora-flujo/cmd/flujo_api/flow.rules.json /cmd/flujo_api/flow.rules.json

# Vault
COPY channel/bin/vault /workspace/channel/bin/vault

COPY framework-echo/frameworkecho /frameworks/frameworkecho
COPY framework-alfa/frameworkalfa /frameworks/frameworkalfa
COPY framework-auditor/frameworkauditor /workspace/framework-auditor/frameworkauditor
COPY framework-auditor/data /workspace/framework-auditor/data
COPY framework-auditor/framework.manifest.json /workspace/framework-auditor/framework.manifest.json
COPY framework-indexa/frameworkindexa /workspace/framework-indexa/frameworkindexa
COPY framework-indexa/data /workspace/framework-indexa/data
COPY framework-sabio/frameworksabio /workspace/framework-sabio/frameworksabio
COPY framework-sabio/framework.manifest.json /workspace/framework-sabio/framework.manifest.json
COPY framework-indexa/framework.manifest.json /workspace/framework-indexa/framework.manifest.json
COPY framework-echo /workspace/framework-echo
COPY framework-alfa /workspace/framework-alfa
COPY framework-foco/foco /workspace/framework-foco/foco
COPY framework-foco/framework.manifest.json /workspace/framework-foco/framework.manifest.json
COPY framework-hosting/frameworkhosting /workspace/framework-hosting/frameworkhosting
COPY framework-hosting/framework.manifest.json /workspace/framework-hosting/framework.manifest.json
COPY framework-mensajero/frameworkmensajero /workspace/framework-mensajero/frameworkmensajero
COPY framework-mensajero/framework.manifest.json /workspace/framework-mensajero/framework.manifest.json
COPY framework-contactos/frameworkcontactos /workspace/framework-contactos/frameworkcontactos
COPY framework-contactos/framework.manifest.json /workspace/framework-contactos/framework.manifest.json
COPY profiles /workspace/profiles
COPY remora-flujo/cmd/flujo_api/entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh /channel /flujo_api \
      /workspace/channel/bin/vault \
      /workspace/framework-indexa/frameworkindexa \
      /workspace/framework-sabio/frameworksabio \
      /workspace/framework-foco/foco \
      /workspace/framework-hosting/frameworkhosting \
      /workspace/framework-mensajero/frameworkmensajero \
      /workspace/framework-contactos/frameworkcontactos \
    && mkdir -p /workspace /sessions /conversations /frameworks \
      /temp/foco /temp/mecanico \
      /workspace/channel/bin \
      /workspace/channel/vault_data \
      /workspace/framework-foco/temp/foco \
      /workspace/framework-sabio/temp \
      /workspace/framework-hosting/temp \
      /workspace/framework-mensajero/temp \
      /workspace/profiles/cobranza-chile

EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
