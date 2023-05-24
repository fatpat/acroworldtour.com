import logging
from http import HTTPStatus
from fastapi import APIRouter, Depends, HTTPException, BackgroundTasks, Response
from typing import List
from fastapi_cache import FastAPICache
from fastapi_cache.decorator import cache

from models.competitions import Competition, CompetitionExport, CompetitionNew, CompetitionState, CompetitionPublicExport, CompetitionPublicExportWithResults
from models.pilots import Pilot
from models.pilots_with_results import PilotWithResults
from models.judges import Judge
from models.teams import Team, TeamExport
from models.seasons import Season, SeasonExport
from models.tricks import Trick
from models.cache import Cache
from core.config import settings
from controllers.utils import UtilsCtrl

log = logging.getLogger(__name__)
public = APIRouter()

# test
@public.get(
    "/test",
    response_class=Response,
)
async def test():
    cache = Cache()
    await Pilot.get(78952, cache=cache)
    await Pilot.get(78952, cache=cache)
    await Pilot.get(78952, cache=cache)
    await Pilot.get(78952, cache=cache)
    log.debug(cache.get('pilots', 78952))
    await Team.getall(deleted=False, cache=cache)
    await Team.getall(deleted=False, cache=cache)
    await Team.getall(deleted=False, cache=cache)
    await Team.get('6335d53bd30964166ad55ed8', cache=cache)

#
# Get all public
#
@public.get(
    "/pilots/",
    response_description="List all public",
    response_model=List[Pilot],
)
@cache(expire=settings.CACHE_EXPIRES)
async def list_pilots():
    cache = Cache()
    return await Pilot.getall(cache=cache)

#
# Get one pilot
#
@public.get(
    "/pilots/{civlid}",
    response_description="Get a Pilot",
    response_model=PilotWithResults,
)
@cache(expire=settings.CACHE_EXPIRES)
async def get_pilot(civlid: int):
    cache = Cache()
    await Pilot.getall(cache=cache)
    await Team.getall(cache=cache)
    await Judge.getall(cache=cache)
    await Trick.getall(cache=cache)
    await Competition.getall(cache=cache)
    await Season.getall(cache=cache)
    return await PilotWithResults.get(civlid, cache=cache)

#
# Get all teams
#
@public.get(
    "/teams/",
    response_description="List all teams",
    response_model=List[TeamExport],
)
@cache(expire=settings.CACHE_EXPIRES)
async def list_teams():
    teams = []
    cache = Cache()
    for team in await Team.getall(deleted=False, cache=cache):
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
@cache(expire=settings.CACHE_EXPIRES)
async def get_team(id: str):
    cache = Cache()
    team = await Team.get(id, cache=cache)
    return await team.export(cache=cache)

#
# Get all judges
#
@public.get(
    "/judges/",
    response_description="List all judges",
    response_model=List[Judge],
)
@cache(expire=settings.CACHE_EXPIRES)
async def list_judges():
    cache = Cache()
    return await Judge.getall(deleted=False, cache=cache)

#
# Get one judge
#
@public.get(
    "/judges/{id}",
    response_description="Get a Judge",
    response_model=Judge,
)
@cache(expire=settings.CACHE_EXPIRES)
async def get_judge(id: str):
    cache = Cache()
    return await Judge.get(id, cache=cache)

#
# Get all competitions
#
@public.get(
    "/competitions/",
    response_description="List all competitions",
    response_model=List[CompetitionPublicExport],
)
@cache(expire=settings.CACHE_EXPIRES)
async def list_competitions():
    cache = Cache()
    comps = []
    for comp in await Competition.getall(cache=cache):
        comp = await comp.export_public(cache=cache)
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
@cache(expire=settings.CACHE_EXPIRES)
async def get_competition(id: str):
    cache = Cache()
    comp = await Competition.get(id, cache=cache)
    return await comp.export_public_with_results(cache=cache)

#
# Get all seasons
#
@public.get(
    "/seasons",
    response_description="List all seasons",
    response_model=List[SeasonExport],
)
async def list_seasons(deleted: bool = False):
    cache = Cache()
    return [await season.export(cache=cache) for season in await Season.getall(deleted=deleted, cache=cache)]

#
# Get a season
#
@public.get(
    "/seasons/{id}",
    response_description="Get a Season",
    response_model=SeasonExport,
)
async def get_season(id: str, deleted: bool = False):
    cache = Cache()
    season = await Season.get(id, deleted=deleted, cache=cache)
    return await season.export(cache=cache)

#
# Get all tricks
#
@public.get(
    "/tricks",
    response_description="List all tricks",
    response_model=List[Trick],
)
async def list(repeatable: bool = None):
    return await Trick.getall(deleted = False, repeatable = repeatable)
