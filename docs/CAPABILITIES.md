# Capabilities Canónicas — Remora

Las **capabilities** son el contrato genérico entre frameworks. Un framework
declara qué `produces` y qué `requires` en su `framework.manifest.json` (campo
`capabilities_semantic`). El orquestador resuelve "quién provee X" leyendo
manifests, **nunca por nombre de framework**.

> Regla de oro: si el orquestador o el frontend menciona el nombre `mecanico`
> o `hosting` para tomar una decisión, está mal. Debe mencionar la
> **capability** (`credentials.smtp`, `message.draft`, etc.) y dejar que el
> registro resuelva.

## Categorías

### `credentials.*` — accesos guardados en el vault

Identifican un secreto persistido (AES-256-GCM) que múltiples frameworks
pueden consumir. La forma del valor es JSON.

| Capability             | Forma del valor                                                                 | Productor típico         | Consumidor típico |
|------------------------|---------------------------------------------------------------------------------|--------------------------|-------------------|
| `credentials.smtp`     | `{host, port, user, pass, from}`                                                | hosting (vía UAPI), user | mensajero         |
| `credentials.imap`     | `{host, port, user, pass}`                                                      | hosting                  | mensajero         |
| `credentials.cpanel`   | `{host, port, user, pass, insecure}`                                            | hosting (login)          | hosting           |
| `credentials.twilio`   | `{account_sid, auth_token, from_number}`                                        | user                     | mensajero         |
| `credentials.whatsapp` | `{phone_number_id, access_token}`                                               | user                     | mensajero         |
| `credentials.stripe`   | `{secret_key}`                                                                  | user                     | (futuro) cobros   |

### `message.*` — comunicación saliente / entrante

| Capability         | Forma del valor                                                  | Productor   | Consumidor |
|--------------------|------------------------------------------------------------------|-------------|------------|
| `message.draft`    | `{channel, subject?, body, to?, metadata}`                       | redactor / mecánico | mensajero  |
| `message.sent`     | `{channel, to, subject?, message_id, sent_at}`                   | mensajero   | (logs)     |
| `message.received` | `{channel, from, subject?, body, received_at}`                   | mensajero   | sabio / triage |

`channel` ∈ `email | sms | whatsapp | telegram` (extensible).

### `data.*` — lectura de datos del negocio

| Capability             | Forma                                            | Productor | Consumidor |
|------------------------|--------------------------------------------------|-----------|------------|
| `data.indexed`         | (intern) vector store / SQL store inicializado   | indexa    | sabio      |
| `data.entity_lookup`   | `{entity, query} → {fields, citations}`         | sabio     | redactor, foco |
| `data.priorities`      | lista priorizada según perfil de negocio         | foco      | UI / sabio |

### `action.*` — operaciones que mutan estado externo

| Capability             | Forma                                            | Productor   | Consumidor |
|------------------------|--------------------------------------------------|-------------|------------|
| `action.fix_proposed`  | `{finding, before, after, rationale}`            | mecánico    | UI         |
| `action.fix_applied`   | `{proposal_id, result}`                          | mecánico    | UI / logs  |
| `action.email_send`    | `{draft} → {message_id}`                         | mensajero   | UI         |

## Cómo se declara

En cada `framework.manifest.json`:

```json
"capabilities_semantic": {
  "produces": ["credentials.smtp", "credentials.imap"],
  "requires": ["credentials.cpanel"]
}
```

## Cómo se resuelve

El orquestador (`api_rest`) escanea todos los manifests al boot y arma:

```
producerOf["credentials.smtp"] = ["hosting"]
consumerOf["credentials.smtp"] = ["mensajero"]
```

Cuando una acción requiere `credentials.smtp` y el vault no la tiene:

1. El orquestador busca `producerOf["credentials.smtp"]` → encuentra `hosting`.
2. Delega el turno conversacional a `hosting` con un hint:
   `{"intent": "provision", "capability": "credentials.smtp"}`.
3. Hosting ejecuta `provision-smtp`, que escribe en el vault.
4. El orquestador reintenta la acción original.

Esto está implementado por la regla genérica `ensure_capability_before_action`
en `flow.rules.json` (perfil-agnóstica).

## Vault: ubicación y formato

Todos los secretos viven bajo:

```
channel/vault_data/<conv_id>/<capability>.enc
```

Cifrado AES-256-GCM con `REMORA_VAULT_KEY` (32 bytes hex/base64). Acceso vía
binario `channel/bin/vault` (subcomandos `get|set|has|list`).

`<conv_id>` puede ser:
- una conversación específica (creds aisladas por chat),
- `_profile_<nombre>` para creds compartidas dentro de un perfil
  (ej. `_profile_cobranza-chile` si todas las cobranzas usan el mismo SMTP).

## Para agregar una nueva capability

1. Documéntala en este archivo (categoría, forma del valor, productor/consumidor).
2. Agrégala al `produces`/`requires` del framework correspondiente.
3. Si es nueva, considera si necesita una entrada en `flow.rules.json` para que
   el orquestador sepa cómo resolverla (suele bastar con la regla genérica
   `ensure_capability_before_action`).
4. **No hardcodees** el nombre de la capability en lógica de UI o
   `flow.rules.json` específicas de un perfil. Esas deben usar referencias
   genéricas como `action.email_send` que indirectamente requieren la creds.
