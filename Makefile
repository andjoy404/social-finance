.PHONY: up down logs migrate-up migrate-down test rebuild clean

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f backend

migrate-up:
	docker compose run --rm migrate up

migrate-down:
	docker compose run --rm migrate down 1

test:
	docker compose run --rm -T backend go test -v ./...

rebuild:
	docker compose down
	docker compose build --no-cache
	docker compose up -d

clean:
	docker compose down -v
	rm -rf backend/tmp
