import logging
from pydantic import ConfigDict, BaseModel, Field, validator
from bson import ObjectId
from typing import List, Optional
from fastapi.encoders import jsonable_encoder
import pymongo
from enum import Enum
from datetime import datetime

from models.pilots import Pilot
from models.teams import Team, TeamExport
from models.tricks import Trick, UniqueTrick
from models.marks import JudgeMark, FinalMark, FinalMarkExport, JudgeMarkExport
from models.cache import Cache

from core.config import settings

log = logging.getLogger(__name__)

class FlightExport(BaseModel):
    pilot: Optional[Pilot] = None
    team: Optional[TeamExport] = None
    tricks: List[UniqueTrick]
    marks: List[JudgeMarkExport]
    did_not_start: bool = False
    final_marks: Optional[FinalMarkExport] = None
    published: bool = False
    warnings: List[str]

class Flight(BaseModel):
    pilot: int
    team: Optional[str] = None
    tricks: List[UniqueTrick]
    marks: List[JudgeMark]
    did_not_start: bool = False
    final_marks: Optional[FinalMark] = None
    published: bool = False
    warnings: List[str]
    model_config = ConfigDict(json_schema_extra={
        "example": {
            "pilot": 1234,
            "tricks": [],
            "marks": [],
            "did_not_start": False,
            "final_marks": {},
            "published": False,
            "warnings": []
        }
    })

    async def export(self, cache:Cache = None) -> FlightExport:

        marks = []
        for mark in self.marks:
            marks.append(await mark.export(cache=cache))

        try:
            pilot = await Pilot.get(self.pilot, cache=cache)
        except:
            pilot = None

        try:
            team = await Team.get(self.team, cache=cache)
            team = await team.export(cache=cache)
        except:
            team = None

        return FlightExport(
            pilot = pilot,
            team = team,
            tricks = self.tricks,
            marks = marks,
            did_not_start = self.did_not_start,
            final_marks = await self.final_marks.export(cache=cache),
            published = self.published,
            warnings = self.warnings,
        )

class FlightNew(BaseModel):
    tricks: List[str]
    marks: List[JudgeMark]
    did_not_start: bool = False
    warnings: List[str] = []
    model_config = ConfigDict(json_schema_extra={
        "example": {
            "tricks": ["LM", "Right Misty Flip"],
            "marks": [],
            "did_not_start": False,
            "warnings": []
        }
    })
