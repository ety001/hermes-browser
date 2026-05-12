.PHONY: build run dev clean lint test tidy deps verify

APP_NAME = hermes-browser

build:
	go build -o $(APP_NAME) ./cmd/server/

run: build
	./$(APP_NAME)

dev:
	go run ./cmd/server/ -c configs/config.yaml

test:
	go test ./... -count=1 -v

clean:
	rm -f $(APP_NAME)
	rm -rf dist/

lint:
	go vet ./...

tidy:
	go mod tidy

deps:
	go mod tidy
	go mod download

verify: build
	@echo "=== Starting server for verification ==="
	@TOKEN="test-verify-token"; \
	echo "Using fixed token for verification: $$TOKEN"; \
	\
	HB_TOKEN=$$TOKEN ./$(APP_NAME) -c configs/config.yaml & \
	SERVER_PID=$$!; \
	sleep 2; \
	\
	echo "=== Health check ==="; \
	curl -s http://127.0.0.1:19875/health; \
	echo ""; \
	\
	echo "=== MCP Initialize ==="; \
	RESP=$$(curl -s -i -X POST http://127.0.0.1:19875/mcp \
	  -H "Authorization: Bearer $$TOKEN" \
	  -H "Content-Type: application/json" \
	  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'); \
	SESSION_ID=$$(echo "$$RESP" | grep -i "Mcp-Session-Id" | awk '{print $$2}' | tr -d '\r'); \
	echo "$$RESP" | tail -1 | head -c 500; \
	echo ""; \
	if [ -n "$$SESSION_ID" ]; then \
		echo "Session ID: $$SESSION_ID"; \
		echo "=== MCP ListTools ==="; \
		curl -s -X POST http://127.0.0.1:19875/mcp \
		  -H "Authorization: Bearer $$TOKEN" \
		  -H "Content-Type: application/json" \
		  -H "Mcp-Session-Id: $$SESSION_ID" \
		  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
		  | head -c 2000; \
		echo ""; \
	else \
		echo "No session ID received from initialize"; \
	fi; \
	\
	kill $$SERVER_PID 2>/dev/null; \
	wait $$SERVER_PID 2>/dev/null; \
	echo "=== Verification complete ==="
