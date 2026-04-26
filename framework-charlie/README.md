# Framework Charlie

Version management framework for Remora.

## Uso

```bash
./charlie status      # Ver estado
./charlie classify    # Clasificar cambios
./charlie next-version # Ver siguiente versión
```

## Reglas

- Si repo limpio: "✅ Repo limpio"
- Si hay cambios: clasificar y proponer mensaje
- No hace commit solo, propone

## Tipos

- `feat`: código nuevo
- `fix`: bug fix
- `docs`: documentación
- `test`: tests
- `chore`: configuración