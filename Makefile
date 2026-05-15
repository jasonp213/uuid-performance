.PHONY: up down build smoke run-1m run-5m run-20m clean reset

up:
	docker compose up -d
	@echo "waiting for db healthchecks…"
	@until [ "$$(docker inspect -f '{{.State.Health.Status}}' uuidlab-pg17 2>/dev/null)" = "healthy" ] && \
	       [ "$$(docker inspect -f '{{.State.Health.Status}}' uuidlab-mysql8 2>/dev/null)" = "healthy" ]; do \
		sleep 2; \
	done
	@echo "both healthy"

down:
	docker compose down

reset:
	docker compose down -v --remove-orphans
	docker compose up -d
	@$(MAKE) up

build:
	go build -o bin/bench .

smoke: build
	./bin/bench -rows 50000 -chunk 25000 -batch 1000 -pagination-limit 50 -output-dir results/smoke

run-1m: build
	./bin/bench -rows 1000000 -chunk 100000 -output-dir results/1m

run-5m: build
	./bin/bench -rows 5000000 -chunk 500000 -output-dir results/5m

run-20m: build
	./bin/bench -rows 20000000 -chunk 1000000 -output-dir results/20m

clean:
	rm -rf bin results
