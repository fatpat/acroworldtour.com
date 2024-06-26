import logging
import json
from http import HTTPStatus
from fastapi import APIRouter, Depends, HTTPException, BackgroundTasks, Response
from typing import List, Any
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

log = logging.getLogger(__name__)
public2 = APIRouter()

#
# Get all public2
#
@public2.get(
    "/pilots/",
    response_description="List all pilots",
    response_model=List[Pilot],
)
async def list_pilots():
    cache = Cache()
    return await Pilot.getall(cache=cache)

#
# Get one pilot
#
@public2.get(
    "/pilots/{civlid}",
    response_description="Get a Pilot",
    response_model=PilotWithResults,
)
async def get_pilot(civlid: int):
    #TODO
    cache = Cache()
    await gather(
        Competition.getall(cache=cache),
        Season.getall(cache=cache),
    )
    return await PilotWithResults.get(civlid, cache=cache)

#
# Get all teams
#
@public2.get(
    "/teams/",
    response_description="List all teams",
    response_model=List[Team],
)
async def list_teams():
    cache = Cache()
    return await Team.getall(deleted=False, cache=cache)

#
# Get one team
#
@public2.get(
    "/teams/{id}",
    response_description="Get a Team",
    response_model=Team,
)
async def get_team(id: str):
    cache = Cache()
    return await Team.get(id, cache=cache)

#
# Get all judges
#
@public2.get(
    "/judges/",
    response_description="List all judges",
    response_model=List[Judge],
)
async def list_judges():
    cache = Cache()
    return await Judge.getall(deleted=False, cache=cache)

#
# Get one judge
#
@public2.get(
    "/judges/{id}",
    response_description="Get a Judge",
    response_model=Judge,
)
async def get_judge(id: str):
    cache = Cache()
    return await Judge.get(id, cache=cache)

#
# Get all competitions
#
@public2.get(
    "/competitions/",
    response_description="List all competitions",
    response_model=List[CompetitionPublicExport],
)
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
@public2.get(
    "/competitions/{id}",
    response_description="Get a Competition",
    response_model=CompetitionPublicExportWithResults,
)
async def get_competition(id: str):
    cache = Cache()
    await gather(
        Pilot.getall(cache=cache),
        Team.getall(cache=cache),
        Judge.getall(cache=cache),
        Trick.getall(cache=cache),
    )
    comp = await Competition.get(id, cache=cache)
    return await comp.export_public2_with_results(cache=cache)

#
# export competition overall standing in SVG
#
@public2.get(
    "/competitions/{id}/standings/overall/svg",
    response_description="export competition overall standing in SVG",
    response_class=Response,
)
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
@public2.get(
    "/competitions/{id}/standings/run/{run}/svg",
    response_description="export competition overall standing in SVG",
    response_class=Response,
)
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
@public2.get(
    "/seasons/",
    response_description="List all seasons",
    response_model=List[SeasonPublicExport],
)
async def list_seasons(deleted: bool = False):
    cache = Cache()
    await gather(
        Pilot.getall(cache=cache),
        Team.getall(cache=cache),
        Judge.getall(cache=cache),
        Competition.getall(cache=cache),
        Trick.getall(cache=cache),
    )
    return [await season.export_public2(cache=cache) for season in await Season.getall(deleted=deleted, cache=cache)]

#
# Get a season
#
@public2.get(
    "/seasons/{id}",
    response_description="Get a Season",
    response_model=SeasonPublicExport,
)
async def get_season(id: str, deleted: bool = False):
    cache = Cache()
    await gather(
        Pilot.getall(cache=cache),
        Team.getall(cache=cache),
        Judge.getall(cache=cache),
        Trick.getall(cache=cache),
    )
    season = await Season.get(id, deleted=deleted, cache=cache)
    return await season.export_public2(cache=cache)

#
# export season standing in SVG
#
@public2.get(
    "/seasons/{id}/standings/svg",
    response_description="export season standing in SVG",
    response_class=Response,
)
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
@public2.get(
    "/tricks/",
    response_description="List all tricks",
    response_model=List[Trick],
)
async def list(repeatable: bool = None):
    return await Trick.getall(deleted = False, repeatable = repeatable)
