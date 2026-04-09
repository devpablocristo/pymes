#!/usr/bin/env python3
"""
Convierte un dump SQL de WooCommerce (WordPress) a CSV compatible con
el importador de productos de pymes-core (/v1/data-io/import/products).

Uso:
    python3 -u scripts/woocommerce_to_pymes_csv.py ~/Downloads/startlap_wp_qbduv.sql -o startlap_productos.csv

El script:
  1. Parsea INSERT INTO de posts (type=product), postmeta, terms, term_taxonomy, term_relationships
  2. Cruza precio, SKU, stock, categorías, marcas, imágenes (guid de attachments)
  3. Genera CSV con columnas: name, type, sku, price, cost_price, unit, tax_rate, track_stock,
     description, tags, image_urls (una URL por línea; también acepta comas al importar)

Progreso: usar python3 -u para ver mensajes sin buffer; el dump grande tarda varios minutos.
"""
import argparse
import csv
import html
import re
import sys
from collections import defaultdict


def parse_inserts(sql_path: str, table_suffix: str, progress_label: str = "") -> list[tuple]:
    """Extrae filas INSERT INTO para una tabla dada (por sufijo, ej. '_posts')."""
    label = progress_label or table_suffix
    pattern = re.compile(
        r"INSERT INTO `[^`]*" + re.escape(table_suffix) + r"` \([^)]+\) VALUES\((.+)\);$"
    )
    rows = []
    with open(sql_path, encoding="utf-8", errors="replace") as f:
        for line_no, line in enumerate(f, 1):
            if line_no % 200_000 == 0:
                print(f"    [{label}] {line_no} líneas leídas, {len(rows)} filas coincidentes...", flush=True)
            m = pattern.match(line.rstrip("\n"))
            if m:
                rows.append(parse_values(m.group(1)))
    print(f"    [{label}] listo: {len(rows)} filas", flush=True)
    return rows


def parse_values(raw: str) -> tuple:
    """Parsea la parte VALUES(...) de un INSERT respetando strings con comas."""
    values = []
    i = 0
    n = len(raw)
    while i < n:
        if raw[i] == "'":
            # string literal
            j = i + 1
            buf = []
            while j < n:
                if raw[j] == "\\" and j + 1 < n:
                    buf.append(raw[j + 1])
                    j += 2
                elif raw[j] == "'":
                    break
                else:
                    buf.append(raw[j])
                    j += 1
            values.append("".join(buf))
            i = j + 1
            # avanzar hasta la coma o fin
            while i < n and raw[i] in (" ", ","):
                if raw[i] == ",":
                    i += 1
                    break
                i += 1
        elif raw[i] in (" ", ","):
            i += 1
        else:
            # numero o NULL
            j = i
            while j < n and raw[j] not in (",",):
                j += 1
            token = raw[i:j].strip()
            values.append(None if token.upper() == "NULL" else token)
            i = j + 1
    return tuple(values)


def image_urls_for_product(
    post_id: str,
    meta_by_post: dict[str, dict[str, str]],
    attachment_guids: dict[str, str],
) -> list[str]:
    """URLs en orden: miniatura (_thumbnail_id) y luego galería (_product_image_gallery)."""
    m = meta_by_post.get(post_id, {})
    ordered_ids: list[str] = []
    seen_ids: set[str] = set()
    thumb = (m.get("_thumbnail_id") or "").strip()
    if thumb.isdigit():
        ordered_ids.append(thumb)
        seen_ids.add(thumb)
    gallery_raw = (m.get("_product_image_gallery") or "").strip()
    if gallery_raw:
        for part in gallery_raw.split(","):
            aid = part.strip()
            if not aid.isdigit() or aid in seen_ids:
                continue
            seen_ids.add(aid)
            ordered_ids.append(aid)
    urls: list[str] = []
    seen_urls: set[str] = set()
    for aid in ordered_ids:
        g = (attachment_guids.get(aid) or "").strip()
        if not g or g in seen_urls:
            continue
        seen_urls.add(g)
        urls.append(g)
    return urls


def strip_html(text: str) -> str:
    """Elimina tags HTML y decodifica entidades."""
    if not text:
        return ""
    text = re.sub(r"<[^>]+>", " ", text)
    text = html.unescape(text)
    text = re.sub(r"\s+", " ", text).strip()
    return text


def main():
    parser = argparse.ArgumentParser(description="WooCommerce SQL dump → Pymes CSV")
    parser.add_argument("sql_file", help="Ruta al dump SQL de WordPress")
    parser.add_argument("-o", "--output", default="productos_importar.csv", help="Archivo CSV de salida")
    parser.add_argument("--tax-rate", type=float, default=21.0, help="Tasa de IVA por defecto (default: 21)")
    args = parser.parse_args()

    print(f"Parseando {args.sql_file}...", flush=True)

    # 1. Posts (productos + attachments para guid)
    print("  [1/5] Tabla _posts...", flush=True)
    print("      (en dumps phpMyAdmin a veces _posts viene al final: ver 0 filas al inicio es normal)", flush=True)
    posts_raw = parse_inserts(args.sql_file, "_posts", progress_label="posts")
    # Columnas: ID(0), ..., post_content(4), post_title(5), ..., post_status(7), ...,
    #           ..., guid(18), ..., post_type(20), post_mime_type(21), ...
    products = {}
    attachment_guids: dict[str, str] = {}
    for row in posts_raw:
        if len(row) < 23:
            continue
        post_type = row[20]
        if post_type == "product" and row[7] == "publish":
            post_id = row[0]
            products[post_id] = {
                "name": strip_html(row[5] or ""),
                "description": strip_html(row[4] or ""),
            }
        elif post_type == "attachment" and row[0] and row[18]:
            attachment_guids[row[0]] = (row[18] or "").strip()
    print(f"  Productos publicados: {len(products)}", flush=True)
    print(f"  Attachments con guid: {len(attachment_guids)}", flush=True)

    # 2. Postmeta (precio, SKU, stock, imágenes)
    print("  [2/5] Tabla _postmeta...", flush=True)
    meta_raw = parse_inserts(args.sql_file, "_postmeta", progress_label="postmeta")
    # Columnas: meta_id(0), post_id(1), meta_key(2), meta_value(3)
    meta_keys = {
        "_regular_price",
        "_sale_price",
        "_price",
        "_sku",
        "_stock",
        "_stock_status",
        "_thumbnail_id",
        "_product_image_gallery",
    }
    meta: dict[str, dict[str, str]] = defaultdict(dict)
    for row in meta_raw:
        if len(row) >= 4 and row[2] in meta_keys and row[1] in products:
            meta[row[1]][row[2]] = row[3] or ""
    print(f"  Metadatos por producto cargados (incl. imágenes)", flush=True)

    # 3. Terms + term_taxonomy + term_relationships (categorías, marcas, tags)
    print("  [3/5] Tabla _terms...", flush=True)
    terms_raw = parse_inserts(args.sql_file, "_terms", progress_label="terms")
    # term_id(0), name(1), slug(2), term_group(3)
    term_names = {}
    for row in terms_raw:
        if len(row) >= 2:
            term_names[row[0]] = row[1]

    print("  [4/5] Tabla _term_taxonomy...", flush=True)
    tax_raw = parse_inserts(args.sql_file, "_term_taxonomy", progress_label="term_taxonomy")
    # term_taxonomy_id(0), term_id(1), taxonomy(2), description(3), parent(4), count(5)
    taxonomy_map: dict[str, tuple[str, str]] = {}  # taxonomy_id → (taxonomy_type, term_name)
    for row in tax_raw:
        if len(row) >= 3 and row[1] in term_names:
            taxonomy_map[row[0]] = (row[2], term_names[row[1]])

    print("  [5/5] Tabla _term_relationships...", flush=True)
    rel_raw = parse_inserts(args.sql_file, "_term_relationships", progress_label="term_relationships")
    # object_id(0), term_taxonomy_id(1), term_order(2)
    product_cats: dict[str, list[str]] = defaultdict(list)
    product_brands: dict[str, list[str]] = defaultdict(list)
    product_tags: dict[str, list[str]] = defaultdict(list)
    for row in rel_raw:
        if len(row) >= 2 and row[0] in products and row[1] in taxonomy_map:
            tax_type, term_name = taxonomy_map[row[1]]
            if tax_type == "product_cat":
                product_cats[row[0]].append(term_name)
            elif tax_type in ("yith_product_brand", "product_brand"):
                product_brands[row[0]].append(term_name)
            elif tax_type == "product_tag":
                product_tags[row[0]].append(term_name)
    print(f"  Categorías asignadas: {sum(len(v) for v in product_cats.values())}", flush=True)
    print(f"  Marcas asignadas: {sum(len(v) for v in product_brands.values())}", flush=True)
    print(f"  Tags asignados: {sum(len(v) for v in product_tags.values())}", flush=True)

    # 4. Generar CSV
    fieldnames = [
        "name",
        "type",
        "sku",
        "price",
        "cost_price",
        "unit",
        "tax_rate",
        "track_stock",
        "description",
        "tags",
        "image_urls",
    ]
    written = 0
    skipped_no_price = 0

    with open(args.output, "w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()

        for post_id, prod in products.items():
            m = meta.get(post_id, {})
            price = m.get("_regular_price", "") or m.get("_price", "")
            if not price:
                skipped_no_price += 1
                continue

            # Limpiar precio
            try:
                price_val = float(price.replace(",", "."))
            except ValueError:
                skipped_no_price += 1
                continue

            sku = m.get("_sku", "")
            stock = m.get("_stock", "")
            has_stock = stock not in ("", None, "NULL")

            # Tags: combinamos categorías + marcas + tags de WooCommerce
            all_tags = []
            all_tags.extend(product_cats.get(post_id, []))
            all_tags.extend(product_brands.get(post_id, []))
            all_tags.extend(product_tags.get(post_id, []))
            # Deduplicar manteniendo orden
            seen = set()
            unique_tags = []
            for t in all_tags:
                t_clean = t.strip()
                if t_clean and t_clean.lower() not in seen:
                    seen.add(t_clean.lower())
                    unique_tags.append(t_clean)

            # Descripción: truncar a 500 chars para no sobrecargar
            desc = prod["description"][:500] if prod["description"] else ""

            img_urls = image_urls_for_product(post_id, meta, attachment_guids)
            # Una URL por línea (el formulario/import aceptan también comas)
            image_urls_cell = "\n".join(img_urls)

            writer.writerow({
                "name": prod["name"],
                "type": "product",
                "sku": sku,
                "price": f"{price_val:.2f}",
                "cost_price": "",
                "unit": "unidad",
                "tax_rate": f"{args.tax_rate:.2f}",
                "track_stock": "true" if has_stock else "false",
                "description": desc,
                "tags": ",".join(unique_tags),
                "image_urls": image_urls_cell,
            })
            written += 1

    print(f"\nResultado:", flush=True)
    print(f"  Productos exportados: {written}", flush=True)
    print(f"  Saltados (sin precio): {skipped_no_price}", flush=True)
    print(f"  Archivo: {args.output}", flush=True)


if __name__ == "__main__":
    main()
