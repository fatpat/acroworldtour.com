import logging
from models.events import Event

log = logging.getLogger(__name__)

class EventCtrl:
    @staticmethod
    def start():
        Event.createIndexes()
