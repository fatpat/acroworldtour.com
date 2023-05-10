FROM python:3.10-alpine

RUN apk update && apk add --no-cache build-base libffi-dev
RUN python -m pip install --upgrade pip

EXPOSE 8000
WORKDIR /app
CMD ["sh", "-c", "python -m venv .venv-docker && . .venv-docker/bin/activate && pip install -r requirements.txt && uvicorn main:app --host 0.0.0.0 --reload --reload-dir ./controllers --reload-dir ./core --reload-dir ./models --reload-dir ./routers"]
