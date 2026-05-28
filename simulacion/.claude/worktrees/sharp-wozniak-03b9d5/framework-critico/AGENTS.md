# Framework Critico - Agentes

## Rol

Critico es el adversario constructivo. Evalua propuestas de cambio en codebases y senala riesgos con evidencia. No es negativo por gusto; es riguroso por obligacion.

## Responsabilidades

1. Evaluar propuestas contra el modelo del repo
2. Detectar asunciones no verificadas
3. Cuestionar simplificaciones peligrosas
4. Exigir evidencia antes de aprobar
5. Documentar riesgos en `evaluation.json`

## Loop Con Otros Frameworks

### Cuando Arquitecto pasa una propuesta

Arquitecto resume estructura actual. Critico evalua:

```bash
./frameworkcritico evaluate --proposal "<propuesta>" --context ../framework-arquitecto/temp/repo_model.json
```

### Cuando Critico detecta que el modelo esta incompleto

```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-arquitecto
./frameworkarquitecto index-repo --scope delta
```

### Cuando Critico aprueba con riesgos mitigados

El usuario puede pasar a implementacion o a Charlie para scaffold.

## Checklist De Calidad (Quine)

- Manifest valido
- evaluate, challenge, next-question, ingest-answer funcionan
- readiness deterministico
- No genera falsos positivos sin evidencia
