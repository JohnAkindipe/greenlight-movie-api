# ==================================================================================== #
# HELPERS
# ==================================================================================== #
## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## confirm: confirm the action to be taken
.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #
## ssh-start: start SSH agent and add private key
.PHONY: ssh-start
ssh-start:
	@eval $$(ssh-agent -s) && ssh-add ~/.ssh/id_rsa_greenlight

## run/api: run the cmd/api application
.PHONY: run/api
run/api:
	cd cmd/api && go run .

## db/migrations/new name=$1: create a new database migration
.PHONY: db/migrations/new
db/migrations/new: confirm
	@echo 'Creating migration files for ${name}...'
	migrate create -seq -ext=.sql -dir=./migrations ${name}

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #
## audit: tidy  and vendor dependencies and format, vet and test all code
.PHONY: audit
audit: vendor
	@echo 'Formatting code...'
	go fmt ./...
	@echo 'Vetting code...'
	go vet ./...
	staticcheck ./...
	@echo 'Running tests...'
	go test -race -vet=off ./...

## vendor: tidy and vendor dependencies
.PHONY: vendor
vendor:
	@echo 'Tidying and verifying module dependencies...'
	go mod tidy
	go mod verify
	@echo 'Vendoring dependencies...'
	go mod vendor
# ==================================================================================== #
# BUILD
# ==================================================================================== #
## build/api: build the cmd/api application
.PHONY: build/api
build/api:
	@echo 'Building cmd/api...'
	go build -ldflags='-s -w' -o=./bin/api ./cmd/api
	GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o=./bin/linux_amd64/api ./cmd/api

# ==================================================================================== #
# PRODUCTION
# ==================================================================================== #
production_host_ip = '64.227.38.217'
## production/connect: connect to the production server
.PHONY: production/connect
production/connect:
	ssh greenlight@${production_host_ip}

## production/deploy/api: deploy the api to production
.PHONY: production/deploy/api
production/deploy/api:
	scp ./bin/linux_amd64/api greenlight@${production_host_ip}:~
	scp -r ./migrations greenlight@${production_host_ip}:~
	scp ./remote/production/api.service greenlight@${production_host_ip}:~
	scp ./remote/production/Caddyfile greenlight@${production_host_ip}:~
	ssh -t greenlight@${production_host_ip} '\
	 migrate -path ~/migrations -database $$GREENLIGHT_DB_DSN up \
	 && sudo mv ~/api.service /etc/systemd/system/ \
	 && sudo systemctl enable api \
	 && sudo systemctl restart api \
	 && sudo mv ~/Caddyfile /etc/caddy/ \
	 && sudo systemctl reload caddy \
	'

	
