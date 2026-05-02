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

WORKDIR /

COPY remora-flujo/cmd/flujo_api/flujo_api /flujo_api
COPY remora-flujo/cmd/flujo_api/channel /channel
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
COPY remora-flujo/cmd/flujo_api/entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh /channel /flujo_api /workspace/framework-indexa/frameworkindexa /workspace/framework-sabio/frameworksabio && mkdir -p /workspace /sessions /conversations /frameworks

EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
