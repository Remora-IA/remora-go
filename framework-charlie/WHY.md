# WHY - Framework Charlie

Charlie existe porque el código necesita versionado, releases y scaffolding
que no dependan de la memoria del desarrollador.

Charlie gestiona el ciclo de vida del código: doctor, plan, propose, validate,
publish, backup. No genera automatizaciones de negocio: eso es Alfa → Bravo.
Charlie cuida el repositorio.

## Problema Que Resuelve

Sin Charlie, cada release es artesanal: commits sin tag, changelogs manuales,
deploys sin preflight. Charlie estandariza ese ciclo con comandos
deterministas.

## Relación Con Otros Frameworks

- **Foco** define qué versión se quiere lograr hoy.
- **Deployer** ejecuta el deploy que Charlie prepara.
- **Paladin** audita la calidad estructural del repo.

Charlie no despliega. Deployer no versiona. Cada uno hace su parte.
