import logging
from http import HTTPStatus
from fastapi import APIRouter, Depends, Body, HTTPException
from core.security import auth
from models.tricks import Trick, UniqueTrick
from typing import List
from fastapi.responses import Response
from controllers.tricks import TrickCtrl
from core.config import settings

log = logging.getLogger(__name__)
tricks = APIRouter()

#
# Get all tricks
#
@tricks.get(
    "/",
    response_description="List all tricks",
    response_model=List[Trick],
    dependencies=[Depends(auth)],
)
async def list(deleted: bool = False, repeatable: bool = None):
    return await Trick.getall(deleted = deleted, repeatable = repeatable)

#
# Get all unique tricks
#
@tricks.get(
    "/uniques",
    response_description="Get all unique tricks",
    response_model=List[UniqueTrick],
    dependencies=[Depends(auth)],
)
async def get_unique_tricks(solo: bool=True, synchro: bool=True):
    return await Trick.get_unique_tricks(solo, synchro)

#
# Get available bonuses
#
@tricks.get(
    "/bonuses",
    response_description="Get available bonuses",
    response_model=List[dict],
    dependencies=[Depends(auth)],
)
async def get_bonuses():
    return settings.tricks.available_bonuses

#
# Get available directions
#
@tricks.get(
    "/directions",
    response_description="Get available directions",
    response_model=List[dict],
    dependencies=[Depends(auth)],
)
async def get_directions():
    return settings.tricks.available_directions


#
# Get a unique trick
#
@tricks.get(
    "/unique/{id:path}", # https://github.com/tiangolo/fastapi/issues/4390#issuecomment-1019558295
    response_description="Get a Unique Trick",
    response_model=UniqueTrick,
    dependencies=[Depends(auth)],
)
async def get_unique_trick(id):
    return await Trick.get_unique_trick(id)

#
# Get one trick
#
@tricks.get(
    "/{id}",
    response_description="Get a Trick",
    response_model=Trick,
    dependencies=[Depends(auth)],
)
async def get(id: str, deleted: bool = False):
    return await Trick.get(id, deleted)

#
# Create a new Trick
#
@tricks.post(
    "/new",
    status_code=201,
    response_description="Add new Trick",
    response_model=Trick,
    dependencies=[Depends(auth)],
)
async def create(trick: Trick = Body(...)):
    TrickCtrl.generate_tricks(trick)
    await trick.create()

#
# Import Tricks
#
@tricks.post(
    "/import",
    status_code=201,
    response_description="Add new Trick",
    response_model=str,
    dependencies=[Depends(auth)],
)
async def create_import(tricks: List[Trick] = Body(...)):
    errors = []
    count = 0
    for trick in tricks:
        try:
            TrickCtrl.generate_tricks(trick)
            await trick.create()
            count += 1
        except Exception as e:
            errors.append(str(e))
    if len(errors) > 0:
        count_error = len(errors)
        errors = ",".join(errors)
        msg = f"Imported successfully {count} tricks and {count_error} failed: {errors}"
        raise HTTPException(status_code=400, detail=msg)
    return f"Imported successfully {count} tricks without errors"

#
# Update an existing Trick
#
@tricks.put(
    "/{id}",
    status_code=204,
    response_description="Add new Trick",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def update(id: str, trick: Trick = Body(...)):
    TrickCtrl.generate_tricks(trick)
    await Trick.update(id, trick)

#
# Delete a Trick
#
@tricks.delete(
    "/{id}",
    status_code=204,
    response_description="Delete a Trick",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def delete(id: str, restore: bool = False):
    res = await Trick.delete(id, restore)
