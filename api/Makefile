default:
	echo "Default target, doing nothing"

deploy-preprod:
	cd .. && make $@

deploy-prod:
	cd .. && make $@

test:
	(cd .. && docker-compose exec api /app/.venv-docker/bin/python -m pytest --disable-warnings -o log_cli=true -o log_cli_level=INFO)
