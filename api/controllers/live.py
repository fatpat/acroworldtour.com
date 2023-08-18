import logging
from tempfile import NamedTemporaryFile
import lxml.html
import re
from fastapi import HTTPException

from models.pilots import Pilot
from models.teams import Team

log = logging.getLogger(__name__)

class LiveCtrl:
    @staticmethod
    async def pilot_overlay(civlid:int, run:int):
        pilot = await Pilot.get(civlid)
        with open('templates/live-overlay-run-pilot.svg') as file:
            svg = file.read()
            return pilot.name, svg.replace("%{RUN}", f"RUN {run}").replace("%{PILOT}", f"{pilot.name.upper()}")

    @staticmethod
    async def team_overlay(team:str, run:int):
        team = await Team.get(team)
        with open('templates/live-overlay-run-pilot.svg') as file:
            svg = file.read()
            return team.name, svg.replace("%{RUN}", f"RUN {run}").replace("%{PILOT}", f"{team.name.upper()}")
