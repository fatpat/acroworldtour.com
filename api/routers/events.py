import logging
from http import HTTPStatus
from fastapi import APIRouter, Depends, Body, HTTPException
from core.security import auth
from models.events import Event
from typing import List
from fastapi.responses import Response

log = logging.getLogger(__name__)
events = APIRouter()

#
# Get all events
#
@events.get(
    "/",
    response_description="List all events",
    response_model=List[Event],
    dependencies=[Depends(auth)],
)
async def list(deleted: bool = False):
    return await Event.getall(deleted)

#
# Get one event
#
@events.get(
    "/{id}",
    response_description="Get a Event",
    response_model=Event,
    dependencies=[Depends(auth)],
)
async def get(id: str, deleted: bool = False):
    return await Event.get(id, deleted)

#
# Create a new Event
#
@events.post(
    "/new",
    status_code=201,
    response_description="Add new Event",
    response_model=Event,
    dependencies=[Depends(auth)],
)
async def create(event: Event = Body(...)):
    return await event.create()

#
# Update an existing Event
#
@events.patch(
    "/{id}",
    status_code=204,
    response_description="Update an Event",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def update(id: str, event: Event = Body(...)):
    await Event.update(id, event)

#
# Delete a Event
#
@events.delete(
    "/{id}",
    status_code=204,
    response_description="Delete a Event",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def delete(id: str, restore: bool = False):
    res = await Event.delete(id, restore)
