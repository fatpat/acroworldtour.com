import logging
from models.teams import Team

log = logging.getLogger(__name__)

class TeamCtrl:
    @staticmethod
    def start():
        Team.createIndexes()
