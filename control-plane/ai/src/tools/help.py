from __future__ import annotations

FAQ = {
    "devolucion": "Para cargar una devolucion: busca la venta, crea la devolucion parcial o total, y define metodo de reembolso.",
    "stock": "Podes ajustar stock desde Inventario > Ajustar. El sistema registra movimiento y auditoria.",
    "turnos": "Los turnos se gestionan en la seccion de citas: disponibilidad, confirmacion y cierre.",
}


async def search_help_docs(query: str) -> dict:
    q = query.lower().strip()
    for key, text in FAQ.items():
        if key in q:
            return {"answer": text}
    return {"answer": "No encontre una guia exacta. Proba con palabras clave: devolucion, stock, turnos."}
