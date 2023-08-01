import logging
from pydantic import BaseModel, Field, validator
from bson import ObjectId
from typing import List, Optional
from fastapi.encoders import jsonable_encoder
import pymongo
from enum import Enum
from datetime import datetime

from models.flights import Flight, FlightExport
from models.pilots import Pilot
from models.teams import Team, TeamExport
from models.cache import Cache

from core.config import settings
from core.utils import float3digits

log = logging.getLogger(__name__)

class RunResultsExport(BaseModel):
    final: bool
    type: str
    results: dict[str, List[FlightExport]]

    class Config:
        json_encoders = {ObjectId: str}

class RunResults(BaseModel):
    final: bool
    type: str
    results: dict[str, List[Flight]]

    class Config:
        schema_extra = {
            "example": {
                "final": False,
                "results": []
            }
        }

    async def export(self, cache:Cache = None) -> FlightExport:
        results = {}
        for result_type in self.results:
            results[result_type] = []
            for result in self.results[result_type]:
                results[result_type].append(await result.export(cache=cache))

        return RunResultsExport(
            final = self.final,
            type = self.type,
            results = results,
        )

class RunResultSummary(BaseModel):
    rank: int
    score: float

    _score = validator('score', allow_reuse=True)(float3digits)

    class Config:
        schema_extra = {
            "example": {
                "rank": "1",
                "score": 12.5
            }
        }

class CompetitionPilotResultsExport(BaseModel):
    pilot: Optional[Pilot]
    team: Optional[TeamExport]
    result_per_run: List[RunResultSummary]
    score: float

    class Config:
        json_encoders = {ObjectId: str}

class CompetitionPilotResults(BaseModel):
    pilot: int
    team: str
    result_per_run: List[RunResultSummary]
    score: float

    _score = validator('score', allow_reuse=True)(float3digits)

    class Config:
        schema_extra = {
            "example": {
                "pilot": "1234",
                "result_per_run": [
                    {
                        "rank": "1",
                        "score": 12.5
                    },
                    {
                        "rank": "3",
                        "score": 11
                    }
                ],
                "score": 23.5,
            }
        }

    async def export(self, cache:Cache = None) -> CompetitionPilotResultsExport:
        try:
          pilot = await Pilot.get(self.pilot, cache=cache)
        except:
          pilot = None

        try:
          team = await Team.get(self.team, cache=cache)
          team = await team.export(cache=cache)
        except Exception as e:
          team = None

        return CompetitionPilotResultsExport(
            pilot = pilot,
            team = team,
            result_per_run = self.result_per_run,
            score = self.score,
        )

class CompetitionResultsExport(BaseModel):
    final: bool
    type: str
    results: dict[str, List[CompetitionPilotResultsExport]]
    runs_results: List[RunResultsExport]

    class Config:
        json_encoders = {ObjectId: str}

class CompetitionResults(BaseModel):
    final: bool
    type: str
    results: dict[str, List[CompetitionPilotResults]]
    runs_results: List[RunResults]

    class Config:
        schema_extra = {
            "example": {
                "final": False,
                "results": {
                    "overall" : []
                },
                "runs_results": []
            }
        }

    async def export(self, cache:Cache = None) -> CompetitionResultsExport:
        results = {}
        for result_type in self.results:
            results[result_type] = []
            for r in self.results[result_type]:
                results[result_type].append(await r.export(cache=cache))

        runs_results = []
        for r in self.runs_results:
            runs_results.append(await r.export(cache=cache))

        return CompetitionResultsExport(
            final = self.final,
            type = self.type,
            results = results,
            runs_results = runs_results,
        )
