# https://hub.docker.com/_/python
FROM python:3.10-alpine

ENV PYTHONUNBUFFERED True
ENV APP_HOME /app
WORKDIR $APP_HOME
COPY . ./

RUN apk add --update-cache build-base
RUN pip install --no-cache-dir -r requirements.txt

CMD ["gunicorn", "main:app", "--workers", "1", "--worker-class", "uvicorn.workers.UvicornWorker", "-b", "0.0.0.0:8080","--forwarded-allow-ips", "*", "--access-logfile", "-"]
