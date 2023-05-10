import logging
from http import HTTPStatus
from fastapi import APIRouter, Depends, HTTPException, BackgroundTasks, Response
from typing import List
from fastapi_cache import FastAPICache
from fastapi_cache.decorator import cache

from models.competitions import Competition, CompetitionExport, CompetitionNew, CompetitionState, CompetitionPublicExport, CompetitionPublicExportWithResults
from models.pilots import Pilot
from models.judges import Judge
from models.teams import Team, TeamExport
from models.seasons import Season, SeasonExport
from core.config import settings
from controllers.utils import UtilsCtrl

log = logging.getLogger(__name__)
public = APIRouter()

#
# Get all public
#
@public.get(
    "/pilots/",
    response_description="List all public",
    response_model=List[Pilot],
)
#@cache(expire=settings.CACHE_EXPIRES)
async def list_teams():
    return await Pilot.getall()

#
# Get one pilot
#
@public.get(
    "/pilots/{civlid}",
    response_description="Get a Pilot",
    response_model=Pilot,
)
#@cache(expire=settings.CACHE_EXPIRES)
async def get_pilot(civlid: int):
    return await Pilot.get(civlid)

#
# Get all teams
#
@public.get(
    "/teams/",
    response_description="List all teams",
    response_model=List[TeamExport],
)
#@cache(expire=settings.CACHE_EXPIRES)
async def list_teams():
    teams = []
    cache = await UtilsCtrl.get_cache()
    for team in await Team.getall(False):
        teams.append(await team.export(cache=cache))
    return teams

#
# Get one team
#
@public.get(
    "/teams/{id}",
    response_description="Get a Team",
    response_model=TeamExport,
)
#@cache(expire=settings.CACHE_EXPIRES)
async def get_team(id: str):
    team = await Team.get(id, False)
    return await team.export(cache=await UtilsCtrl.get_cache())

#
# Get all judges
#
@public.get(
    "/judges/",
    response_description="List all judges",
    response_model=List[Judge],
)
#@cache(expire=settings.CACHE_EXPIRES)
async def list_judges():
    return await Judge.getall(False)

#
# Get one judge
#
@public.get(
    "/judges/{id}",
    response_description="Get a Judge",
    response_model=Judge,
)
#@cache(expire=settings.CACHE_EXPIRES)
async def get_judge(id: str):
    return await Judge.get(id, False)

#
# Get all competitions
#
@public.get(
    "/competitions/",
    response_description="List all competitions",
    response_model=List[CompetitionPublicExport],
)
#@cache(expire=settings.CACHE_EXPIRES)
async def list_competitions():
    comps = []
    for comp in await Competition.getall():
        comp = await comp.export_public()
        if comp is not None:
            comps.append(comp)
    return comps

#
# Get one competition
#
@public.get(
    "/competitions/{id}",
    response_description="Get a Competition",
    response_model=CompetitionPublicExportWithResults,
)
#@cache(expire=settings.CACHE_EXPIRES)
async def get_competition(id: str):
    comp = await Competition.get(id)
    return await comp.export_public_with_results(cache=await UtilsCtrl.get_cache())

#
# Get all seasons
#
@public.get(
    "/seasons",
    response_description="List all seasons",
    response_model=List[SeasonExport],
)
async def list_seasons(deleted: bool = False):
    cache=await UtilsCtrl.get_cache()
    return [await season.export(cache=cache) for season in await Season.getall(deleted=deleted)]

#
# Get a season
#
@public.get(
    "/seasons/{id}",
    response_description="Get a Season",
    response_model=SeasonExport,
)
async def get_season(id: str, deleted: bool = False):
    cache=await UtilsCtrl.get_cache()
    season = await Season.get(id, deleted=deleted, cache=cache)
    return await season.export(cache=cache)
