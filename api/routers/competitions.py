import logging
import os
import re
from datetime import datetime
from http import HTTPStatus
from fastapi import APIRouter, Depends, Body, HTTPException, Request, BackgroundTasks
from typing import List
from fastapi.responses import Response, HTMLResponse, FileResponse
from fastapi.templating import Jinja2Templates

from core.security import auth

from models.competitions import Competition, CompetitionExport, CompetitionNew, CompetitionState, CompetitionPublicExport, CompetitionType
from models.competition_configs import CompetitionConfig
from models.runs import Run, RunExport, RunRepetitionsResetPolicy
from models.marks import FinalMark, FinalMarkExport
from models.flights import Flight, FlightNew, FlightExport
from models.results import RunResults, CompetitionResults, CompetitionResultsExport, RunResultsExport
from models.pilots import Pilot
from models.cache import Cache
from models.seasons import Season
from models.teams import Team
from controllers.competitions import CompCtrl
from controllers.utils import UtilsCtrl

log = logging.getLogger(__name__)
competitions = APIRouter()
templates = Jinja2Templates(directory="templates")

#
# Get all competitions
#
@competitions.get(
    "/",
    response_description="List all competitions",
    response_model=List[CompetitionPublicExport],
    dependencies=[Depends(auth)]
)
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
@competitions.get(
    "/{id}",
    response_description="Get a Competition",
    response_model=CompetitionExport,
    dependencies=[Depends(auth)]
)
async def get_by_id(id: str, deleted: bool = False):
    cache = Cache()
    comp = await Competition.get(id, deleted)
    return await comp.export(cache=cache)

#
# Create a new Competition
#
@competitions.post(
    "/new",
    status_code=201,
    response_description="Add new Competition",
    response_model=CompetitionExport,
    dependencies=[Depends(auth)],
)
async def create(competition: CompetitionNew = Body(...)):
    cache=Cache()
    comp = await competition.create()
    return await comp.export(cache=cache)

#
# Update a Competition
#
@competitions.patch(
    "/{id}",
    status_code=204,
    response_description="Update a competition",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def update(id: str, updated_comp: CompetitionNew = Body(...)):
    comp = await Competition.get(id)
    await comp.update(updated_comp)

#
# Update Pilot list
#
@competitions.patch(
    "/{id}/pilots",
    status_code=204,
    response_description="Replace the pilot's list",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def patch_pilots(id: str, pilots: List[int] = Body(...)):
    comp = await Competition.get(id)
    await comp.update_pilots(pilots)

#
# Update Teams list
#
@competitions.patch(
    "/{id}/teams",
    status_code=204,
    response_description="Replace the teams's list",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def patch_teams(id: str, teams: List[str] = Body(...)):
    comp = await Competition.get(id)
    await comp.update_teams(teams)

#
# Update Judges list
#
@competitions.patch(
    "/{id}/judges",
    status_code=204,
    response_description="Replace the judge's list",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def patch_judges(id: str, judges: List[str] = Body(...)):
    comp = await Competition.get(id)
    await comp.update_judges(judges)

#
# Update Repeatable Tricks list
#
@competitions.patch(
    "/{id}/repeatable_tricks",
    status_code=204,
    response_description="Replace the repeatable tricks list",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def patch_repeatable_tricks(id: str, repeatable_tricks: List[str] = Body(...)):
    comp = await Competition.get(id)
    await comp.update_repeatable_tricks(repeatable_tricks)


#
# Update Config
#
@competitions.patch(
    "/{id}/config",
    status_code=204,
    response_description="Replace the competition config",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def patch_config(id: str, config: CompetitionConfig = Body(...)):
    comp = await Competition.get(id)
    await comp.update_config(config)

@competitions.delete(
    "/{id}",
    status_code=204,
    response_description="Delete a Competition",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def delete(id: str, restore: bool = False):
    await Competition.delete(id, restore)

@competitions.post(
    "/{id}/open",
    status_code=204,
    response_description="Open a competition (change status from not started to open)",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def open(id: str):
    comp = await Competition.get(id)
    await comp.open()

@competitions.post(
    "/{id}/close",
    status_code=204,
    response_description="Close a competition (change status from open to close)",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def close(id: str):
    comp = await Competition.get(id)
    await comp.close()

@competitions.post(
    "/{id}/reopen",
    status_code=204,
    response_description="Reopen a closed competition (change status from close to open)",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def close(id: str):
    comp = await Competition.get(id)
    await comp.reopen()

@competitions.post(
    "/{id}/runs/new",
    status_code=201,
    response_description="Create a new run for a competition",
    response_model=RunExport,
    dependencies=[Depends(auth)],
)
async def new_run(id: str, pilots_to_qualify: int = 0, repetitions_reset_policy: int = RunRepetitionsResetPolicy.none):
    cache = Cache()
    comp = await Competition.get(id, cache=cache)
    log.debug(f"repetitions_reset_policy={repetitions_reset_policy}")
    run = await comp.new_run(pilots_to_qualify, repetitions_reset_policy=repetitions_reset_policy)
    return await run.export(cache=cache)

@competitions.get(
    "/{cid}/runs/{rid}",
    response_description="Retrieve a run",
    response_model=RunExport,
    dependencies=[Depends(auth)],
)
async def get_run(cid: str, rid: int):
    cache=Cache()
    comp = await Competition.get(cid, cache=cache)
    run = await comp.run_get(rid)
    return await run.export(cache=cache)

#
# Update Run Pilot list
#
@competitions.patch(
    "/{id}/runs/{i}/pilots",
    status_code=204,
    response_description="Replace the run pilots list",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def patch_run_pilots(id: str, i: int, pilots: List[int] = Body(...)):
    comp = await Competition.get(id)
    await comp.run_update_pilots(i, pilots)

#
# Update Teams list
#
@competitions.patch(
    "/{id}/runs/{i}/teams",
    status_code=204,
    response_description="Replace the run teams list",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def patch_run_teams(id: str, i: int, teams: List[str] = Body(...)):
    comp = await Competition.get(id)
    await comp.run_update_teams(i, teams)

#
# Update Judges list
#
@competitions.patch(
    "/{id}/runs/{i}/judges",
    status_code=204,
    response_description="Replace the run judges list",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def patch_run_judges(id: str, i: int, judges: List[str] = Body(...)):
    comp = await Competition.get(id)
    await comp.run_update_judges(i, judges)

#
# Update Repeatable Tricks list
#
@competitions.patch(
    "/{id}/runs/{i}/repeatable_tricks",
    status_code=204,
    response_description="Replace the run repeatable tricks list",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def patch_run_repeatable_tricks(id: str, i: int, repeatable_tricks: List[str] = Body(...)):
    comp = await Competition.get(id)
    await comp.run_update_repeatable_tricks(i, repeatable_tricks)


#
# Update Config
#
@competitions.patch(
    "/{id}/runs/{i}/config",
    status_code=204,
    response_description="Replace the run config",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def patch_run_config(id: str, i: int, config: CompetitionConfig = Body(...)):
    comp = await Competition.get(id)
    await comp.run_update_config(i, config)

@competitions.post(
    "/{id}/runs/{i}/open",
    status_code=204,
    response_description="Open a run (change status from not started to open)",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def run_open(id: str, i: int):
    comp = await Competition.get(id)
    await comp.run_open(i)

@competitions.post(
    "/{id}/runs/{i}/close",
    status_code=204,
    response_description="Close a run (change status from not started to open)",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def run_close(id: str, i: int):
    comp = await Competition.get(id)
    await comp.run_close(i)

@competitions.post(
    "/{id}/runs/{i}/reopen",
    status_code=204,
    response_description="Reopen a run (change status from not started to open)",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def run_reopen(id: str, i: int):
    comp = await Competition.get(id)
    await comp.run_reopen(i)

@competitions.get(
    "/{id}/runs/{i}/flights/{pilot_team_id}",
    response_description="retrieve a specific flight from a pilot",
    response_model=Flight,
    dependencies=[Depends(auth)],
)
async def flight_get(id: str, i: int, pilot_team_id):
    log.debug(f"flight_get{id} {i}  {pilot_team_id}")
    comp = await Competition.get(id)
    return await comp.flight_get(run_i= i, pilot_or_team=pilot_team_id)

@competitions.delete(
    "/{id}/runs/{i}/flights/{pilot_team_id}",
    status_code=204,
    response_description="delete a flight from a pilot",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def flight_delete(id: str, i: int, pilot_team_id):
    comp = await Competition.get(id)
    await comp.flight_delete(run_i= i, pilot_or_team=pilot_team_id)


@competitions.post(
    "/{id}/runs/{i}/flights/{pilot_team_id}/new",
    response_description="Simulate a run and get the detail score",
    response_model=FinalMarkExport,
    dependencies=[Depends(auth)],
)
async def flight_save(id: str, i: int, pilot_team_id, save: bool, published:bool = False, mark_type:str = None, flight: FlightNew = Body(...)):
    cache = Cache()
    comp = await Competition.get(id, cache=cache)
    mark = await comp.flight_save(run_i=i, id=pilot_team_id, flight=flight, save=save, published=published, mark_type=mark_type, cache=cache)
    return await mark.export(cache=cache)

@competitions.get(
    "/{id}/results",
    status_code=200,
    response_description="Retrieve the results of the competition",
    response_model=CompetitionResultsExport,
    dependencies=[Depends(auth)],
)
async def get_all_results(id: str):
    cache = Cache()
    comp = await Competition.get(id, cache=cache)
    res = await comp.results()
    return await res.export(cache=cache)

@competitions.get(
    "/{id}/results/export",
    status_code=200,
    response_description="Rietrieve the results of the competition",
    response_class=FileResponse,
#    dependencies=[Depends(auth)],
)
async def get_export_results(request: Request, id: str, bg_tasks: BackgroundTasks, limit_run: int =-1, filetype: str = "xls"):
    cache = Cache()
    comp = await Competition.get(id, cache=cache)
    res = await comp.results(limit = limit_run)

    # skip overall if it's a tour competition
    if filetype != "xls" and any(re.search(r"^aw[tqs]", s) for s in comp.seasons):
        del res.results['overall']

    # if it's a World Acro Championship, no seasons in results
    if len(comp.seasons) == 0:
        for key in list(res.results.keys()).copy():
            if key not in ["overall", "women"]:
                del(res.results[key])

    res = await res.export(cache=cache)
    seasons = {}
    for season in comp.seasons:
        seasons[season] = await Season.get(season)

    if filetype == "xls":
        file = CompCtrl.comp_to_xlsx(res, comp)
    elif filetype == "html":
        return templates.TemplateResponse("comp_results.html", {"request": request, "results":res, "comp":comp, "limit_run":limit_run, "seasons": seasons})
    else:
        raise HTTPException(status_code=400, detail="wrong file type, must be xls or html")
    bg_tasks.add_task(os.remove, file)
    filename=f"{id}-overall-results"
    if limit_run >= 0:
        filename += f"-after-run{limit_run+1}"
    filename += f"-{datetime.now().strftime('%Y-%m-%d-%H%M%S')}.xlsx"
    return FileResponse(path=file, filename=filename, background=bg_tasks)

@competitions.get(
    "/{id}/results/{i}",
    status_code=200,
    response_description="Retrieve the results of a specific run of competition",
    response_model=RunResultsExport,
    dependencies=[Depends(auth)],
)
async def run_get_results(id: str, i: int, published_only: bool = True):
    cache = Cache()
    comp = await Competition.get(id, cache=cache)
    res = await comp.run_results(run_i=i, published_only=published_only)
    return await res.export(cache=cache)

@competitions.get(
    "/{id}/results/{i}/export",
    status_code=200,
    response_description="export the results of a specific run of competition",
    response_class=FileResponse,
#    dependencies=[Depends(auth)],
)
async def run_get_results(request: Request, id: str, i: int, bg_tasks: BackgroundTasks, filetype: str = "xls"):
    cache = Cache()
    comp = await Competition.get(id, cache=cache)
    res = await comp.run_results(run_i=i)

    # skip overall if it's a tour competition
    if filetype != "xls" and any(re.search(r"^aw[tqs]", s) for s in comp.seasons):
        del res.results['overall']

    # if it's a World Acro Championship, no seasons in results
    print(f"seasons={comp.seasons}")
    if len(comp.seasons) == 0:
        for key in list(res.results.keys()).copy():
            if key not in ["overall", "women"]:
                del(res.results[key])

    res = await res.export(cache=cache)

    seasons = {}
    for season in comp.seasons:
        seasons[season] = await Season.get(season)

    if filetype == "xls":
        file = CompCtrl.run_to_xlsx(res, comp)
    elif filetype == "html":
        for result_type in res.results:
            res.results[result_type].sort(key=lambda e: -e.final_marks.score)
        return templates.TemplateResponse("run_results.html", {"request": request, "results":res, "comp":comp, "rid": i, "seasons": seasons})
    else:
        raise HTTPException(status_code=400, detail="wrong file type, must be xls or html")
    bg_tasks.add_task(os.remove, file)
    filename=f"{id}-run{i+1}-results-{datetime.now().strftime('%Y-%m-%d-%H%M%S')}.xlsx"
    return FileResponse(path=file, filename=filename, background=bg_tasks)


@competitions.get(
    "/{id}/starting_order/{i}/export",
    status_code=200,
    response_description="export the starting order of a specific run of competition",
    response_class=FileResponse,
#    dependencies=[Depends(auth)],
)
async def export_starting_order(request: Request, id: str, i: int, bg_tasks: BackgroundTasks, filetype: str = "xls"):
    cache = Cache()
    comp = await Competition.get(id, cache=cache)
    run = comp.runs[i]
    starting_order = {}


    if comp.type == CompetitionType.synchro:
        starting_order["overall"] = []
        for team in run.teams:
            team = await Team.get(team)
            for j, pilot in enumerate(team.pilots):
                pilot = await Pilot.get(pilot)
                team.pilots[j] = pilot
            starting_order["overall"].append(team)
    else:
        if any(re.search(r"^aw[tqs]", s) for s in comp.seasons):
            for season in comp.seasons:
                if season.startswith("awt"):
                    season = await Season.get(season)
                    starting_order[season.name] = []
                    for pilot in run.pilots:
                        pilot = await Pilot.get(pilot)
                        if season.year in pilot.awt_year:
                            starting_order[season.name].append(pilot)

                elif season.startswith("awq"):
                    season = await Season.get(season)
                    starting_order[season.name] = []
                    for pilot in run.pilots:
                        pilot = await Pilot.get(pilot)
                        if season.year not in pilot.awt_years:
                            starting_order[season.name].append(pilot)
        else:
            starting_order["overall"] = []
            for pilot in run.pilots:
                pilot = await Pilot.get(pilot)
                starting_order["overall"].append(pilot)

    log.warning(i)
    return templates.TemplateResponse("run_starting_order.html", {
        "request": request,
        "starting_order": starting_order,
        "comp":comp,
        "rid": i,
        "last_update": datetime.now(),
    })
