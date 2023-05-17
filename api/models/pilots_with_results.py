import logging
from pydantic import BaseModel, Field, validator, HttpUrl
from bson import ObjectId
from enum import Enum
from pycountry import countries
from typing import Optional, List
from datetime import datetime
from fastapi.encoders import jsonable_encoder
import pymongo
from fastapi import HTTPException

from core.database import db, PyObjectId
from models.pilots import Pilot
from models.competitions import CompetitionPublicExport, Competition, CompetitionType
from models.seasons import Season, SeasonExport

log = logging.getLogger(__name__)

class CompetitionResult(BaseModel):
    competition: CompetitionPublicExport
    rank: int

class SeasonResult(BaseModel):
    season: Season
    rank: int

class PilotWithResults(Pilot):
    competitions_results: List[CompetitionResult] = Field([], description="List of competitions results")
    seasons_results: List[SeasonResult] = Field([], description="List of seasons results")

    async def get(id: int, cache:dict ={}):
        pilot = await Pilot.get(id, cache)
        if pilot is None:
            return None
        pilot = PilotWithResults.parse_obj(pilot)

        seasons = []
        for comp in await Competition.getall():
            if not comp.published:
                continue

            if comp.type == CompetitionType.solo and id not in comp.pilots:
                continue

            comp_with_results = await comp.export_public_with_results(cache)

            team = None
            if comp.type == CompetitionType.synchro:
                for t in comp_with_results.teams:
                    if t.pilots[0].id == id or t.pilots[1].id == id:
                        team = t
                        break

                if team is None:
                    continue

            if not comp_with_results.results.final:
                continue

            for rank, result in enumerate(comp_with_results.results.overall_results):
                if comp.type == CompetitionType.solo and result.pilot.id != id:
                    continue

                if comp.type == CompetitionType.synchro and result.team.id != team.id:
                    continue

                competition_results = CompetitionResult(
                    competition = await comp.export_public(),
                    rank = rank+1,
                )
                pilot.competitions_results.append(competition_results)
                seasons += comp.seasons
                break

        seasons = list(set(seasons))
        for season in seasons:
            season = await Season.get(season)
            season_export = await season.export(cache)
            for results in season_export.results:
                if results.type != "overall":
                    continue
                for rank, result in enumerate(results.results):
                    res = None

                    log.debug(f"{season.code} > {result}")

                    if result.pilot is not None and result.pilot.id == id:
                        res = SeasonResult(
                            season = season,
                            rank = rank+1
                        )

                    elif result.team is not None and (result.team.pilots[0].id == id or result.team.pilots[1].id == id):
                        res = SeasonResult(
                            season = season,
                            rank = rank+1
                        )

                    if res is not None:
                        pilot.seasons_results.append(res)
                        break

        return pilot
