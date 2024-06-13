import logging
from http import HTTPStatus
from fastapi import APIRouter, Depends, Body, HTTPException
from typing import List
from fastapi.responses import Response

from core.security import auth
from models.teams import Team, TeamExport
from models.cache import Cache
from controllers.utils import UtilsCtrl

log = logging.getLogger(__name__)
teams = APIRouter()

#
# Get all teams
#
@teams.get(
    "/",
    response_description="List all teams",
    response_model=List[TeamExport],
    dependencies=[Depends(auth)],
)
async def list(deleted: bool = False):
    teams = []
    cache = Cache()
    for team in await Team.getall(deleted, cache=cache):
        teams.append(await team.export(cache=cache))
    return teams

#
# Get one team
#
@teams.get(
    "/{id}",
    response_description="Get a Team",
    response_model=TeamExport,
    dependencies=[Depends(auth)],
)
async def get(id: str, deleted: bool = False):
    cache = Cache()
    team = await Team.get(id, deleted, cache=cache)
    return await team.export(cache=cache)

#
# Create a new Team
#
@teams.post(
    "/new",
    status_code=201,
    response_description="Add new Team",
    response_model=TeamExport,
    dependencies=[Depends(auth)],
)
async def create(team: Team = Body(...)):
    cache = Cache()
    team = await team.create()
    return await team.export(cache=cache)

#
# Update an existing Team
#
@teams.put(
    "/{id}",
    status_code=204,
    response_description="Update an existing Team",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def update(id: str, team: Team = Body(...)):
    await Team.update(id, team)

#
# Delete a Team
#
@teams.delete(
    "/{id}",
    status_code=204,
    response_description="Delete a Team",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def delete(id: str, restore: bool = False):
    await Team.delete(id, restore)
