DC := $(shell command -v docker-compose 2> /dev/null)
WAIT_SERVER_TIME := 7

default: services-down
	@docker-compose up --remove-orphan -d

start: default

install-docker:
ifndef DC
	@echo '----- "docker-compose" is required. https://docs.docker.com/engine/installation/#supported-platforms -----'
	exit 1
endif

services-down: install-docker
	@docker-compose down

success-scenario: default
	@sleep $(WAIT_SERVER_TIME)
	curl http://localhost:4040/healthcheck/

parallel-success-scenario: default
	@sleep $(WAIT_SERVER_TIME)
	curl http://localhost:4040/parallel-healthcheck/

failed-scenario: services-down
	@docker-compose up --remove-orphan -d service
	@sleep $(WAIT_SERVER_TIME)
	curl http://localhost:4040/healthcheck/

parallel-failed-scenario: services-down
	@docker-compose up --remove-orphan -d service
	@sleep $(WAIT_SERVER_TIME)
	curl http://localhost:4040/parallel-healthcheck/

all:
	@echo 'Healthcheck fails' && make failed-scenario 2> /dev/null && echo "\n" && sleep 1
	@echo 'Healthcheck fails [parallel]' && make parallel-failed-scenario 2> /dev/null && echo "\n" && sleep 1
	@echo 'Healthcheck succeed' && make success-scenario  2> /dev/null && echo "\n" && sleep 1
	@echo 'Healthcheck succeed [parallel]' && make parallel-success-scenario  2> /dev/null
