# WHY - Framework Arquitecto

Arquitecto existe porque un repositorio grande no cabe en la cabeza de nadie.

Arquitecto indexa la estructura del repositorio, genera un modelo semántico
del código y permite que otros frameworks (Crítico, Charlie) operen sobre
una representación actualizada sin leer miles de archivos.

## Problema Que Resuelve

Sin Arquitecto, entender la estructura de un repo requiere exploración
manual. Arquitecto genera un mapa actualizado con dependencias, funciones
principales y relaciones entre paquetes.

## Relación Con Otros Frameworks

- **Crítico** evalúa propuestas usando el modelo de Arquitecto.
- **Charlie** puede usar el modelo para decidir qué archivos tocar.
- **Quine** clasifica frameworks usando la estructura que Arquitecto detecta.

Arquitecto no evalúa ni genera código. Solo mapea.
