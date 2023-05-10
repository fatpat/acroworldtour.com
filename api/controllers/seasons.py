import logging
from models.seasons import Season

log = logging.getLogger(__name__)

class SeasonCtrl:
    @staticmethod
    def start():
        Season.createIndexes()
