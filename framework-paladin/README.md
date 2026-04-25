# Framework Paladin

Paladin es el framework de tracing reusable para Remora.

Responsabilidad:

- spans jerarquicos;
- variables;
- decisiones;
- errores;
- snapshots JSON en `temp/paladin`;
- persistencia incremental mientras el proceso corre.

No compara flujo ideal vs flujo real. Esa responsabilidad queda en Framework Bravo.
