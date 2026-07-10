# Seguridad

CompassDTL modela una superficie de liquidacion financiera donde el estado de
riesgo, las reservas y la prioridad de ejecucion deben permanecer coherentes a
lo largo del ciclo de vida de un intent.

## Modelo De Seguridad

- Cada intent debe seleccionar una ruta habilitada, con liquidez operativa y
  limites economicos compatibles con su importe.
- La ejecucion debe conservar el orden definido por prioridad, vencimiento y
  coste estimado.
- Las reservas contables deben balancear debitos, creditos, fees y recibos.
- Los ajustes de exposicion se registran como eventos de reconciliacion.
- Los snapshots deben ser deterministas para auditoria y reproduccion.

## Invariantes Esperadas

- Ningun balance disponible puede ser negativo.
- Las rutas deshabilitadas no aceptan nuevos intents.
- Las comisiones se calculan por activo y por ruta.
- Los recibos de liquidacion deben apuntar a un ticket conocido.
- La exposicion reportada por ruta y corredor debe poder reconciliarse con el
  diario de eventos.

## Validacion

La validacion local ejecuta formato, build, tests Go, tests TypeScript y conteo
de lineas fuente:

```bash
bash scripts/ci.sh
```

Las dependencias de Node se fijan mediante `package-lock.json` cuando se ejecuta
`npm install`. Go usa exclusivamente la biblioteca estandar.

## Alcance De Revision

Quedan dentro de alcance:

- seleccion de rutas;
- calculo de fees;
- limites de exposicion;
- prioridad y liquidacion;
- reconciliacion de eventos;
- handlers HTTP y runner de escenarios.

Quedan fuera de alcance integraciones con custodios reales, firmas externas,
proveedores KYC, redes de pagos productivas y almacenamiento persistente.

## Reporte Interno

Los hallazgos deben incluir escenario reproducible, impacto economico, activos
afectados, invariantes rotas y propuesta de mitigacion verificable mediante
tests automatizados.
