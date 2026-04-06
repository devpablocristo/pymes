#!/usr/bin/env python3
"""
Convierte un dump SQL de WooCommerce (WordPress) a CSV compatible con
el importador de productos de pymes-core (/v1/data-io/import/products).

Uso:
    python3 scripts/woocommerce_to_pymes_csv.py startlap_wp_qbduv.sql -o startlap_productos.csv

El script:
  1. Parsea INSERT INTO de posts (type=product), postmeta, terms, term_taxonomy, term_relationships
  2. Cruza precio, SKU, stock, categorías, marcas
  3. Genera CSV con columnas: name, type, sku, price, cost_price, unit, tax_rate, track_stock, description, tags
"""
import argparse
import csv
import html
import re
import sys
from collections import defaultdict


def parse_inserts(sql_path: str, table_suffix: str) -> list[tuple]:
    """Extrae filas INSERT INTO para una tabla dada (por sufijo, ej. '_posts')."""
    pattern = re.compile(
        r"INSERT INTO `[^`]*" + re.escape(table_suffix) + r"` \([^)]+\) VALUES\((.+)\);$"
    )
    rows = []
    with open(sql_path, encoding="utf-8", errors="replace") as f:
        for line in f:
            m = pattern.match(line.rstrip("\n"))
            if m:
                rows.append(parse_values(m.group(1)))
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

    print(f"Parseando {args.sql_file}...")

    # 1. Posts (productos)
    posts_raw = parse_inserts(args.sql_file, "_posts")
    # Columnas: ID(0), post_author(1), post_date(2), ..., post_title(5), ..., post_status(7),
    #           ..., post_type(20), ...
    # Índices según el CREATE TABLE del dump
    products = {}
    for row in posts_raw:
        if len(row) >= 23 and row[20] == "product" and row[7] == "publish":
            post_id = row[0]
            products[post_id] = {
                "name": strip_html(row[5] or ""),
                "description": strip_html(row[4] or ""),
            }
    print(f"  Productos publicados: {len(products)}")

    # 2. Postmeta (precio, SKU, stock)
    meta_raw = parse_inserts(args.sql_file, "_postmeta")
    # Columnas: meta_id(0), post_id(1), meta_key(2), meta_value(3)
    meta_keys = {"_regular_price", "_sale_price", "_price", "_sku", "_stock", "_stock_status"}
    meta: dict[str, dict[str, str]] = defaultdict(dict)
    for row in meta_raw:
        if len(row) >= 4 and row[2] in meta_keys and row[1] in products:
            meta[row[1]][row[2]] = row[3] or ""
    print(f"  Metadatos relevantes cargados")

    # 3. Terms + term_taxonomy + term_relationships (categorías, marcas, tags)
    terms_raw = parse_inserts(args.sql_file, "_terms")
    # term_id(0), name(1), slug(2), term_group(3)
    term_names = {}
    for row in terms_raw:
        if len(row) >= 2:
            term_names[row[0]] = row[1]

    tax_raw = parse_inserts(args.sql_file, "_term_taxonomy")
    # term_taxonomy_id(0), term_id(1), taxonomy(2), description(3), parent(4), count(5)
    taxonomy_map: dict[str, tuple[str, str]] = {}  # taxonomy_id → (taxonomy_type, term_name)
    for row in tax_raw:
        if len(row) >= 3 and row[1] in term_names:
            taxonomy_map[row[0]] = (row[2], term_names[row[1]])

    rel_raw = parse_inserts(args.sql_file, "_term_relationships")
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
    print(f"  Categorías asignadas: {sum(len(v) for v in product_cats.values())}")
    print(f"  Marcas asignadas: {sum(len(v) for v in product_brands.values())}")
    print(f"  Tags asignados: {sum(len(v) for v in product_tags.values())}")

    # 4. Generar CSV
    fieldnames = ["name", "type", "sku", "price", "cost_price", "unit", "tax_rate", "track_stock", "description", "tags"]
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
            })
            written += 1

    print(f"\nResultado:")
    print(f"  Productos exportados: {written}")
    print(f"  Saltados (sin precio): {skipped_no_price}")
    print(f"  Archivo: {args.output}")


if __name__ == "__main__":
    main()
