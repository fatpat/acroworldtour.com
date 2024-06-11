import logging
import logging.config
from core.config import settings

logging.basicConfig(level = logging.getLevelName(settings.LOG_LEVEL))
