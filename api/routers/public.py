import logging
import json
import re
import unicodedata
import base64
import io
import zipfile
from http import HTTPStatus
from fastapi import APIRouter, Depends, HTTPException, BackgroundTasks, Response, Body
from typing import List, Any
from asyncio import gather

from models.competitions import Competition, CompetitionExport, CompetitionNew, CompetitionState, CompetitionPublicExport, CompetitionPublicExportWithResults, CompetitionType
from models.pilots import Pilot
from models.pilots_with_results import PilotWithResults
from models.judges import Judge
from models.teams import Team, TeamExport
from models.seasons import Season, SeasonPublicExport, SeasonExportLight
from models.tricks import Trick, UniqueTrick
from models.marks import FinalMark
from models.flights import Flight, FlightNew
from models.cache import Cache
from models.files import File
from core.config import settings
from controllers.utils import UtilsCtrl
from controllers.seasons import SeasonCtrl
from controllers.competitions import CompCtrl
from controllers.scores import ScoreCtrl
from controllers.live import LiveCtrl

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
    "/competitions/{id}/standings/{result_type}/svg",
    response_description="export competition overall standing in SVG",
    response_class=Response,
)
async def export_competition_overall_standing_svg(id: str, result_type: str, download: bool = False, animated: int=-1):
    cache = Cache()
    await gather(
        Pilot.getall(cache=cache),
        Team.getall(cache=cache),
        Judge.getall(cache=cache),
        Trick.getall(cache=cache),
    )
    competition = await Competition.get(id, deleted=False, cache=cache)
    results = await competition.results()
    svg = CompCtrl.svg_overall(competition=await results.export(cache=cache), comp=competition, result_type=result_type, animated=animated)

    headers = {}
    if download:
        headers["Content-Disposition"] = f"attachment; filename=\"{season.code}.{type}.standing.svg\""
    else:
        headers["Content-Disposition"] = f"inline"

    return Response(content=svg, media_type="image/svg+xml", headers=headers)

#
# export competition run standing in SVG
#
@public.get(
    "/competitions/{id}/standings/run/{run}/svg",
    response_description="export competition run standing in SVG",
    response_class=Response,
)
@public.get(
    "/competitions/{id}/standings/{result_type}/run/{run}/svg",
    response_description="export competition run standing in SVG",
    response_class=Response,
)
async def export_competition_overall_standing_svg(id: str, run: int, result_type: str='overall', download: bool = False, animated: int = -1):
    cache = Cache()
    await gather(
        Pilot.getall(cache=cache),
        Team.getall(cache=cache),
        Judge.getall(cache=cache),
        Trick.getall(cache=cache),
    )
    competition = await Competition.get(id, deleted=False, cache=cache)
    results = await competition.results()
    svg = CompCtrl.svg_run(competition=await results.export(cache=cache), comp=competition, run=run, result_type=result_type, animated=animated)

    headers = {}
    if download:
        headers["Content-Disposition"] = f"attachment; filename=\"{season.code}.run{run}.{result_type}.standing.svg\""
    else:
        headers["Content-Disposition"] = f"inline"

    return Response(content=svg, media_type="image/svg+xml", headers=headers)

#
# Get all seasons
#
@public.get(
    "/seasons/",
    response_description="List all seasons",
    response_model=List[SeasonExportLight],
)
async def list_seasons(deleted: bool = False):
    cache = Cache()
    return [await season.export_light(cache=cache) for season in await Season.getall(deleted=deleted, cache=cache)]

#
# Get a season
#
@public.get(
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
    return await season.export_public(cache=cache)

#
# export season standing in SVG
#
@public.get(
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
@public.get(
    "/tricks/",
    response_description="List all tricks",
    response_model=List[Trick],
)
async def list(repeatable: bool = None):
    return await Trick.getall(deleted = False, repeatable = repeatable)

#
# Get all tricks
#
@public.get(
    "/tricks/unique/",
    response_description="List all unique tricks",
    response_model=List[UniqueTrick],
)
async def list(solo: bool = True, synchro: bool = False):
    return await Trick.get_unique_tricks(solo = solo, synchro = synchro)

#
# Single Score Simulation
#
@public.post(
    "/simulate/competition/{t}",
    response_model=List[Flight],
)
async def simulate(t: CompetitionType, flights: List[FlightNew] = Body(...), reset_repetitions_frequency:int = 0):
    return await ScoreCtrl.simulate_scores(flights=flights, type=t, reset_repetitions_frequency=reset_repetitions_frequency)

#
# Get live template for pilot
#
@public.get(
    "/live/overlay/pilot/{civlid}/run/{run}",
    response_class=Response,
)
async def live_overlay_pilot(civlid:int, run:int, download:bool = False):
    pilot, svg = await LiveCtrl.pilot_overlay(civlid, run)
    pilot = ''.join(c for c in unicodedata.normalize('NFD', pilot) if unicodedata.category(c) != 'Mn')
    pilot = re.sub(r'[^\x00-\x7f]',r'', pilot)

    headers = {}
    if download:
        headers["Content-Disposition"] = f"attachment; filename=\"live.overlay.run{run}.pilot.{pilot}.svg\""
    else:
        headers["Content-Disposition"] = f"inline"

    return Response(content=svg, media_type="image/svg+xml", headers=headers)

#
# Get live template for team
#
@public.get(
    "/live/overlay/team/{team}/run/{run}",
    response_class=Response,
)
async def live_overlay_team(team:str, run:int, download:bool = False):
    team, svg = await LiveCtrl.team_overlay(team, run)
    team = ''.join(c for c in unicodedata.normalize('NFD', team) if unicodedata.category(c) != 'Mn')
    team = re.sub(r'[^\x00-\x7f]',r'', team)

    headers = {}
    if download:
        headers["Content-Disposition"] = f"attachment; filename=\"live.overlay.team.run{run}.{team}.svg\""
    else:
        headers["Content-Disposition"] = f"inline"

    return Response(content=svg, media_type="image/svg+xml", headers=headers)

#
# get all templates for a competition
#
@public.get(
    "/live/overlay/competition/{id}",
    response_class=Response,
)
async def live_overlay_competition(id:str,n_run:int = 10):

    cache = Cache()
    competition = await Competition.get(id, deleted=False, cache=cache)

    files=[]
    if competition.type == CompetitionType.solo:
        for civlid in competition.pilots:
            for run in range(1, n_run+1):
                pilot, svg = await LiveCtrl.pilot_overlay(civlid, run)
                pilot = ''.join(c for c in unicodedata.normalize('NFD', pilot) if unicodedata.category(c) != 'Mn')
                pilot = re.sub(r'[^\x00-\x7f]',r'', pilot)
                filename = f"live.overlay.pilot.run{run}.pilot.{pilot}.svg"
                files.append((filename, svg))
    else:
        for t in competition.teams:
            for run in range(1, n_run+1):
                team, svg = await LiveCtrl.team_overlay(t, run)
                team = ''.join(c for c in unicodedata.normalize('NFD', team) if unicodedata.category(c) != 'Mn')
                team = re.sub(r'[^\x00-\x7f]',r'', team)
                filename = f"live.overlay.team.run{run}.team.{team}.svg"
                files.append((filename, svg))


    zip_buffer = io.BytesIO()
    with zipfile.ZipFile(zip_buffer, "a", zipfile.ZIP_DEFLATED, False) as zip_file:
        for filename, svg in files:
            zip_file.writestr(filename, svg)

    headers = {}
    headers["Content-Disposition"] = f"attachment; filename=\"live.overlay.competition.{id}.zip\""
    return Response(content=zip_buffer.getvalue(), media_type="application/zip", headers=headers)
#
# GET files
#
@public.get(
    "/files/{id}",
    response_description="",
    response_class=Response,
)
async def get_file(id: str, download : bool = False):
    file = await File.get(id)
    headers = {}
    if download:
        headers["Content-Disposition"] = f"attachment; filename=\"{file.filename}\""
    else:
        headers["Content-Disposition"] = f"inline"

    return Response(content=base64.b64decode(file.content), media_type=file.content_type, headers=headers)
