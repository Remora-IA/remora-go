APP := autonomia-controlada
DB_PATH ?= ./panalbit.sqlite
PORT ?= 8091

.PHONY: build up run down restart

build:
	go build -o $(APP) .

up: build
	PANALBIT_DB_PATH=$(DB_PATH) PORT=$(PORT) GROQ_API_KEY=$(GROQ_API_KEY) ./$(APP)

run: up

down:
	@pids=$$(lsof -tiTCP:$(PORT) -sTCP:LISTEN 2>/dev/null); \
	if [ -n "$$pids" ]; then \
		echo "Stopping processes on port $(PORT): $$pids"; \
		kill $$pids; \
	else \
		echo "No process listening on port $(PORT)"; \
	fi

restart: down up
