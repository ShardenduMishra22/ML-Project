SHELL := /bin/bash

ifneq (,$(wildcard ./.env))
include .env
export
endif

COMPOSE_FILE ?= docker-compose.yml
DC := docker compose --env-file .env -p $(COMPOSE_PROJECT_NAME) -f $(COMPOSE_FILE)

.PHONY: build up down restart logs clean migrate seed test integration-test

build:
	$(DC) build --pull

up:
	$(DC) up -d --build
	$(MAKE) migrate
	$(MAKE) seed

down:
	$(DC) down --remove-orphans

restart: down up

logs:
	$(DC) logs -f --tail=200

clean:
	$(DC) down -v --remove-orphans

migrate:
	$(DC) run --rm backend migrate

seed:
	$(DC) run --rm backend seed

test:
	cd backend-go && go test ./...
	$(DC) run --rm --workdir /app ml-service sh -lc 'PYTHONPATH=/app pytest -q'

integration-test:
	bash ./tests/integration/full_pipeline.sh
