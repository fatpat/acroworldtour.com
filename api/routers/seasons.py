import logging
from http import HTTPStatus
from fastapi import APIRouter, Depends, Body, HTTPException
from core.security import auth
from models.seasons import Season, SeasonExport
from typing import List
from fastapi.responses import Response

from controllers.utils import UtilsCtrl

log = logging.getLogger(__name__)
seasons = APIRouter()

#
# Get all seasons
#
@seasons.get(
    "/",
    response_description="List all seasons",
    response_model=List[SeasonExport],
    dependencies=[Depends(auth)],
)
async def list(deleted: bool = False):
    cache=await UtilsCtrl.get_cache()
    return [await season.export(cache=cache) for season in await Season.getall(deleted=deleted)]

#
# Get one season
#
@seasons.get(
    "/{id}",
    response_description="Get a Season",
    response_model=Season,
    dependencies=[Depends(auth)],
)
async def get(id: str, deleted: bool = False):
    cache=await UtilsCtrl.get_cache()
    return await Season.get(id, deleted=deleted, cache=cache)

#
# Get one export season
#
@seasons.get(
    "/{id}/export",
    response_description="Get a Season",
    response_model=SeasonExport,
    dependencies=[Depends(auth)],
)
async def export(id: str, deleted: bool = False):
    cache=await UtilsCtrl.get_cache()
    season = await Season.get(id, deleted=deleted, cache=cache)
    return await season.export(cache=cache)

#
# Create a new Season
#
@seasons.post(
    "/new",
    status_code=201,
    response_description="Add new Season",
    response_model=Season,
    dependencies=[Depends(auth)],
)
async def create(season: Season = Body(...)):
    return await season.create()

#
# Update an existing Season
#
@seasons.put(
    "/{id}",
    status_code=204,
    response_description="update Season",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def update(id: str, season: Season = Body(...)):
    await Season.update(id, season)

#
# Delete a Season
#
@seasons.delete(
    "/{id}",
    status_code=204,
    response_description="Delete a Season",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def delete(id: str, restore: bool = False):
    res = await Season.delete(id, restore)
