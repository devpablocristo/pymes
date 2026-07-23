V2_DIR := v2

.DEFAULT_GOAL := help

.PHONY: help up down ps logs smoke ci \
	backend-run backend-test \
	db-up db-down db-migrate db-test db-integration \
	web-dev web-ci

help:
	@echo "Pymes v2"
	@echo ""
	@echo "  make up              Construye y levanta postgres, migrate, backend y web"
	@echo "  make smoke           Verifica base, API y web"
	@echo "  make ps              Muestra el estado del stack"
	@echo "  make logs            Sigue los logs"
	@echo "  make down            Detiene el stack sin borrar el volumen"
	@echo "  make ci              Ejecuta los checks nativos de v2"
	@echo "  make db-integration  Prueba migraciones y reinicio de PostgreSQL"

up down ps logs smoke ci \
backend-run backend-test \
db-up db-down db-migrate db-test db-integration \
web-dev web-ci:
	$(MAKE) -C $(V2_DIR) $@
