

VERSION:=$(shell gitversion /output json /showvariable FullSemVer)
include deploy/.env
export
run:
	go run -ldflags "-s -w -X 'github.com/danstis/replicator/internal/version.Version=$(VERSION)'" cmd/replicator/main.go

up:
	docker compose --project-directory deploy up -d --build --remove-orphans

version:
	@echo $(VERSION)
