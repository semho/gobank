build:
	@go build -o bin/gobank

run: build 
	@./bin/gobank

test:
	@go test -v ./...

docker-build:
	docker run --name gobank-postgres -e POSTGRES_PASSWORD=pass_for_gobank -p 5432:5432 -d postgres

docker-run:
	docker start gobank-postgres