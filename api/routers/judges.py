import logging
from http import HTTPStatus
from fastapi import APIRouter, Depends, Body, HTTPException
from core.security import auth
from models.judges import Judge, JudgeLevel
from typing import List
from fastapi.responses import Response

log = logging.getLogger(__name__)
judges = APIRouter()

#
# Get all judges
#
@judges.get(
    "/",
    response_description="List all judges",
    response_model=List[Judge],
    dependencies=[Depends(auth)],
)
async def list(deleted: bool = False):
    return await Judge.getall(deleted)

#
# get Judges levels
#
@judges.get(
    "/levels",
    response_description="Get list of judges levels",
    response_model=List[JudgeLevel],
    dependencies=[Depends(auth)],
)
def get_levels():
    return [level.value for level in JudgeLevel]

#
# Get one judge
#
@judges.get(
    "/{id}",
    response_description="Get a Judge",
    response_model=Judge,
    dependencies=[Depends(auth)],
)
async def get(id: str, deleted: bool = False):
    return await Judge.get(id, deleted)

#
# Create a new Judge
#
@judges.post(
    "/new",
    status_code=201,
    response_description="Add new Judge",
    response_model=Judge,
    dependencies=[Depends(auth)],
)
async def create(judge: Judge = Body(...)):
    return await judge.create()

#
# Update an existing Judge
#
@judges.put(
    "/{id}",
    status_code=204,
    response_description="Add new Judge",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def update(id: str, judge: Judge = Body(...)):
    await Judge.update(id, judge)

#
# Delete a Judge
#
@judges.delete(
    "/{id}",
    status_code=204,
    response_description="Delete a Judge",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def delete(id: str, restore: bool = False):
    res = await Judge.delete(id, restore)
