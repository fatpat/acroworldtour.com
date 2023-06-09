import logging
import json
from http import HTTPStatus
from fastapi import APIRouter, Depends, HTTPException, BackgroundTasks, Response
from typing import List, Any
from fastapi_cache import FastAPICache
from fastapi_cache.decorator import cache
from asyncio import gather

from models.competitions import Competition, CompetitionExport, CompetitionNew, CompetitionState, CompetitionPublicExport, CompetitionPublicExportWithResults
from models.pilots import Pilot
from models.pilots_with_results import PilotWithResults
from models.judges import Judge
from models.teams import Team, TeamExport
from models.seasons import Season, SeasonPublicExport
from models.tricks import Trick
from models.cache import Cache
from core.config import settings
from controllers.utils import UtilsCtrl
from controllers.seasons import SeasonCtrl
from controllers.competitions import CompCtrl
from core.utils import GenericResponseCoder

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
    await gather(
        Pilot.getall(cache=cache),
        Team.getall(cache=cache),
        Judge.getall(cache=cache),
        Trick.getall(cache=cache),
        Competition.getall(cache=cache),
        Season.getall(cache=cache),
    )
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
    await Pilot.getall(cache=cache),
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
    await Pilot.getall(cache=cache),
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
    await gather(
        Pilot.getall(cache=cache),
        Team.getall(cache=cache),
        Judge.getall(cache=cache),
        Trick.getall(cache=cache),
    )
    comp = await Competition.get(id, cache=cache)
    return await comp.export_public_with_results(cache=cache)

#
# export competition overall standing in SVG
#
@public.get(
    "/competitions/{id}/standings/overall/svg",
    response_description="export competition overall standing in SVG",
    response_class=Response,
)
@cache(expire=settings.CACHE_EXPIRES, coder=GenericResponseCoder)
async def export_competition_overall_standing_svg(id: str, download: bool = False):
    cache = Cache()
    await gather(
        Pilot.getall(cache=cache),
        Team.getall(cache=cache),
        Judge.getall(cache=cache),
        Trick.getall(cache=cache),
    )
    competition = await Competition.get(id, deleted=False, cache=cache)
    results = await competition.results()
    svg = CompCtrl.svg_overall(await results.export(cache=cache))

    headers = {}
    if download:
        headers["Content-Disposition"] = f"attachment; filename=\"{season.code}.standing.svg\""
    else:
        headers["Content-Disposition"] = f"inline"

    return Response(content=svg, media_type="image/svg+xml", headers=headers)

#
# export competition run standing in SVG
#
@public.get(
    "/competitions/{id}/standings/run/{run}/svg",
    response_description="export competition overall standing in SVG",
    response_class=Response,
)
@cache(expire=settings.CACHE_EXPIRES, coder=GenericResponseCoder)
async def export_competition_overall_standing_svg(id: str, run: int, download: bool = False):
    cache = Cache()
    await gather(
        Pilot.getall(cache=cache),
        Team.getall(cache=cache),
        Judge.getall(cache=cache),
        Trick.getall(cache=cache),
    )
    competition = await Competition.get(id, deleted=False, cache=cache)
    results = await competition.results()
    svg = CompCtrl.svg_run(await results.export(cache=cache), run)

    headers = {}
    if download:
        headers["Content-Disposition"] = f"attachment; filename=\"{season.code}.standing.svg\""
    else:
        headers["Content-Disposition"] = f"inline"

    return Response(content=svg, media_type="image/svg+xml", headers=headers)

#
# Get all seasons
#
@public.get(
    "/seasons/",
    response_description="List all seasons",
    response_model=List[SeasonPublicExport],
)
@cache(expire=settings.CACHE_EXPIRES)
async def list_seasons(deleted: bool = False):
    cache = Cache()
    await gather(
        Pilot.getall(cache=cache),
        Team.getall(cache=cache),
        Judge.getall(cache=cache),
        Competition.getall(cache=cache),
        Trick.getall(cache=cache),
    )
    return [await season.export_public(cache=cache) for season in await Season.getall(deleted=deleted, cache=cache)]

#
# Get a season
#
@public.get(
    "/seasons/{id}",
    response_description="Get a Season",
    response_model=SeasonPublicExport,
)
@cache(expire=settings.CACHE_EXPIRES)
async def get_season(id: str, deleted: bool = False):
    cache = Cache()
    await gather(
        Pilot.getall(cache=cache),
        Team.getall(cache=cache),
        Judge.getall(cache=cache),
        Trick.getall(cache=cache),
    )
    season = await Season.get(id, deleted=deleted, cache=cache)
    return await season.export_public(cache=cache)

#
# export season standing in SVG
#
@public.get(
    "/seasons/{id}/standings/svg",
    response_description="export season standing in SVG",
    response_class=Response,
)
@cache(expire=settings.CACHE_EXPIRES, coder=GenericResponseCoder)
async def export_season_standing_svg(id: str, download: bool = False):
    cache = Cache()
    await gather(
        Pilot.getall(cache=cache),
        Team.getall(cache=cache),
        Judge.getall(cache=cache),
        Trick.getall(cache=cache),
    )
    season = await Season.get(id, deleted=False, cache=cache)
    svg = SeasonCtrl.svg(await season.export(cache=cache))

    headers = {}
    if download:
        headers["Content-Disposition"] = f"attachment; filename=\"{season.code}.standing.svg\""
    else:
        headers["Content-Disposition"] = f"inline"

    return Response(content=svg, media_type="image/svg+xml", headers=headers)

#
# Get all tricks
#
@public.get(
    "/tricks/",
    response_description="List all tricks",
    response_model=List[Trick],
)
@cache(expire=settings.CACHE_EXPIRES)
async def list(repeatable: bool = None):
    return await Trick.getall(deleted = False, repeatable = repeatable)
