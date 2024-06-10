import logging
from pydantic import ConfigDict, BaseModel, Field, validator
from bson import ObjectId
from typing import List, Optional
from fastapi.encoders import jsonable_encoder
import pymongo
from enum import Enum
from datetime import datetime

from models.flights import Flight, FlightExport
from models.competition_configs import CompetitionConfig
from models.pilots import Pilot
from models.teams import Team, TeamExport
from models.judges import Judge
from models.tricks import Trick
from models.cache import Cache

from core.config import settings

log = logging.getLogger(__name__)

class RunState(str, Enum):
    init = 'init'
    open = 'open'
    closed = 'closed'

class RunRepetitionsResetPolicy(int, Enum):
    none = 0
    awt = 1
    awq = 2
    all = 3

class RunExport(BaseModel):
    state: RunState
    pilots: List[Pilot]
    teams: List[TeamExport]
    judges: List[Judge]
    repeatable_tricks: List[Trick]
    config: CompetitionConfig
    flights: List[FlightExport]
    repetitions_reset_policy: RunRepetitionsResetPolicy = Field(RunRepetitionsResetPolicy.none)
    # TODO[pydantic]: The following keys were removed: `json_encoders`.
    # Check https://docs.pydantic.dev/dev-v2/migration/#changes-to-config for more information.
    model_config = ConfigDict(json_encoders={ObjectId: str})

class Run(BaseModel):
    state: RunState
    pilots: List[int] = Field(..., min_len=1)
    teams: List[str] = Field(..., min_len=1)
    judges: List[str] = Field(..., min_len=1)
    repeatable_tricks: List[str]
    config: CompetitionConfig
    flights: List[Flight]
    repetitions_reset_policy: RunRepetitionsResetPolicy = Field(RunRepetitionsResetPolicy.none)
    model_config = ConfigDict(json_schema_extra={
        "example": {
            "state": "init",
            "pilots": [1234, 4567],
            "teams": [],
            "judges": ["bb1726576153281283", "ba789798798798798798"],
            "repeatable_tricks": [],
            "config": {
                "warning": 0.5,
                "malus_repetition": 13
            },
            "flights": []
        }
    })

    async def export(self, cache:Cache = None) -> RunExport:

        pilots = []
        for pilot in self.pilots:
            pilots.append(await Pilot.get(pilot, cache=cache))

        teams = []
        for team in self.teams:
            team = await Team.get(team, cache=cache)
            teams.append(await team.export(cache=cache))

        judges = []
        for judge in self.judges:
            judges.append(await Judge.get(judge, cache=cache))

        repeatable_tricks = []
        for trick in self.repeatable_tricks:
            repeatable_tricks.append(await Trick.get(trick, cache=cache, deleted=(self.state == RunState.closed)))

        flights = []
        for flight in self.flights:
            flights.append(await flight.export(cache=cache))

        return RunExport(
            state=self.state,
            pilots=pilots,
            teams=teams,
            judges=judges,
            repeatable_tricks=repeatable_tricks,
            config=self.config,
            flights=flights,
            repetitions_reset_policy=self.repetitions_reset_policy,
        )
