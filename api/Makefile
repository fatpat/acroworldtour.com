default:
	echo "Default target, doing nothing"

deploy-preprod:
	cd .. && make $@

deploy-prod:
	cd .. && make $@

test:
	(cd .. && docker-compose exec back /app/.venv-docker/bin/python -m pytest)
