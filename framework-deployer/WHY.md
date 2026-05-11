# WHY - Framework Deployer

Deployer existe porque un deploy manual a Cloud Run es un proceso de 15 pasos
que nadie recuerda completo.

Deployer ejecuta plan → preflight → build → deploy → verify. Nunca toca
producción salvo que se le diga explícitamente. Solo DEV por defecto.

## Problema Que Resuelve

Sin Deployer, cada deploy es artesanal: comandos copiados, variables
olvidadas, verificaciones saltadas. Deployer estandariza el flujo y verifica
que el servicio esté vivo después de desplegar.

## Relación Con Otros Frameworks

- **Charlie** prepara el código y la versión antes del deploy.
- **Foco** puede incluir un deploy como tarea del día.
- **Paladin** audita que el Dockerfile y las configuraciones sean correctas.

Deployer no versiona código. Charlie no despliega. Cada uno hace su parte.
