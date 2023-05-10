import logging
import logging.config

logging.config.dictConfig({
    "version": 1,
    "root": {
        "level": "DEBUG",
        "handlers": ["console"]
    },
    "handlers": {
        "console": {
            "class": "logging.StreamHandler",
            "level": "DEBUG",
            "formatter": "simple",
            "stream": "ext://sys.stdout"
        }
    },
    "loggers": {
        "uvicorn": {
            "error": {
                "propagate": True
            }
        }
    },
    "formatters": {
        "simple": {
            "format": '%(levelname)s [%(name)s] %(message)s'
        }
    }
})
