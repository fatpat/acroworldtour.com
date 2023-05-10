import logging
from models.judges import Judge

log = logging.getLogger(__name__)

class JudgeCtrl:
    @staticmethod
    def start():
        Judge.createIndexes()
