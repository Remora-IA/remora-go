# CPGs del Proyecto

Esta carpeta contiene los Code Property Graphs (CPGs) de los proyectos Go.

## CPGs Disponibles (4)

| Proyecto | Tamaño | Métodos | Descripción |
|----------|--------|---------|-------------|
| remora-flujo | 4.1MB | 1068 | API REST principal de Flujo |
| remora-cli | 586KB | 124 | CLI del cliente Remora |
| channel | 418KB | 129 | Adaptador de Canal (Brave/Safe) |
| framework-alfa | 369KB | 99 | Framework Alfa (análisis de oportunidades) |

## Uso

```bash
# Listar métodos de un CPG
joern-list-methods --cpg .pi/cpgs/remora-flujo-cpg.bin

# Consultar el CPG con queries
joern-query-cpg --cpg .pi/cpgs/remora-flujo-cpg.bin "cpg.method.name.l"

# Encontrar sinks (operaciones peligrosas)
joern-find-sinks --cpg .pi/cpgs/remora-flujo-cpg.bin --sink-type exec

# Ver call graph de un método
joern-callgraph --cpg .pi/cpgs/remora-flujo-cpg.bin --method-name runFlow
```

## nota

Los nuevos parseos con `joern-parse` CLI fallan (None.get en generateCpg).
Los CPGs existentes funcionan correctamente a través de la API del plugin pi.
Los frameworks sin CPGs necesitan una solución alternativa (ej: joern v1.1+).
