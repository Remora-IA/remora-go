# Cloud Run deployment for api_rest + Channel + Frameworks
# Opción A: binaries pre-compilados
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ENV API_REST_PORT=8080
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

COPY remora-flujo/cmd/api_rest/api_rest /api_rest
COPY remora-flujo/cmd/api_rest/channel /channel
COPY remora-flujo/cmd/api_rest/flow.rules.json /cmd/api_rest/flow.rules.json

# Vault
COPY channel/bin/vault /workspace/channel/bin/vault

COPY framework-echo/frameworkecho /frameworks/frameworkecho
COPY framework-alfa/frameworkalfa /frameworks/frameworkalfa
COPY framework-auditor/frameworkauditor /workspace/framework-auditor/frameworkauditor
COPY framework-auditor/data /workspace/framework-auditor/data
COPY framework-auditor/framework.manifest.json /workspace/framework-auditor/framework.manifest.json
COPY framework-indexa/frameworkindexa /workspace/framework-indexa/frameworkindexa
COPY framework-indexa/data /workspace/framework-indexa/data
COPY framework-sabio /workspace/framework-sabio
COPY framework-indexa/framework.manifest.json /workspace/framework-indexa/framework.manifest.json
COPY framework-echo /workspace/framework-echo
COPY framework-alfa /workspace/framework-alfa
COPY framework-arquitecto /workspace/framework-arquitecto
COPY framework-critico /workspace/framework-critico
COPY framework-deployer /workspace/framework-deployer
COPY framework-foco/foco /workspace/framework-foco/foco
COPY framework-foco/framework.manifest.json /workspace/framework-foco/framework.manifest.json
COPY framework-radar/frameworkradar /workspace/framework-radar/frameworkradar
COPY framework-radar/framework.manifest.json /workspace/framework-radar/framework.manifest.json
COPY framework-hosting/frameworkhosting /workspace/framework-hosting/frameworkhosting
COPY framework-hosting/framework.manifest.json /workspace/framework-hosting/framework.manifest.json
COPY framework-mecanico /workspace/framework-mecanico
COPY framework-mensajero/frameworkmensajero /workspace/framework-mensajero/frameworkmensajero
COPY framework-mensajero/framework.manifest.json /workspace/framework-mensajero/framework.manifest.json
COPY framework-paladin /workspace/framework-paladin
COPY profiles /workspace/profiles
COPY remora-flujo/cmd/api_rest/entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh /channel /api_rest \
      /workspace/channel/bin/vault \
      /workspace/framework-arquitecto/frameworkarquitecto \
      /workspace/framework-critico/frameworkcritico \
      /workspace/framework-deployer/deployer \
      /workspace/framework-indexa/frameworkindexa \
      /workspace/framework-sabio/frameworksabio \
      /workspace/framework-foco/foco \
      /workspace/framework-radar/frameworkradar \
      /workspace/framework-hosting/frameworkhosting \
      /workspace/framework-mecanico/frameworkmecanico \
      /workspace/framework-mensajero/frameworkmensajero \
      /workspace/framework-paladin/frameworkpaladin \
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
