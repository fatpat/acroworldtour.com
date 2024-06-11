import logging
from pydantic import ConfigDict, BaseModel, Field, validator, AnyHttpUrl
from bson import ObjectId
from enum import Enum
from pycountry import countries
from typing import Optional, List
from fastapi.encoders import jsonable_encoder
import pymongo
from datetime import date, datetime
from fastapi import HTTPException

from core.database import db, PyObjectId
from core.config import settings

from models.competitions import Competition, CompetitionPublicExport
from models.cache import Cache

log = logging.getLogger(__name__)
collection = db.events

def check_country(cls, v):
    assert countries.get(alpha_3=v) is not None, f"Invalid country '{v}'"
    return v


class EventExport(BaseModel):
    id: str = Field(alias="_id")
    name: str = Field(..., description="The full name of the event", min_length=2)
    code: str = Field(..., description="Code of the event", min_length=2)
    start_date: date
    end_date: date
    country: str = Field(..., description="The country of the event using the 3 letter acronym of the country")
    location: str = Field(..., description="The place of the event", min_length=2)
    website: Optional[AnyHttpUrl] = None
    description: Optional[str] = None
    image_url: Optional[AnyHttpUrl | str] = None
    logo_url: Optional[AnyHttpUrl | str] = None
    streaming_url: Optional[AnyHttpUrl] = None
    competitions: List[CompetitionPublicExport]
    # TODO[pydantic]: The following keys were removed: `json_encoders`.
    # Check https://docs.pydantic.dev/dev-v2/migration/#changes-to-config for more information.
    model_config = ConfigDict(populate_by_name=True, arbitrary_types_allowed=True, json_encoders={ObjectId: str})

class Event(BaseModel):
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    name: str = Field(..., description="The full name of the event", min_length=2)
    code: str = Field(..., description="Code of the event", min_length=2)
    start_date: date
    end_date: date
    country: str = Field(..., description="The country of the event using the 3 letter acronym of the country")
    location: str = Field(..., description="The place of the event", min_length=2)
    website: Optional[AnyHttpUrl] = None
    description: Optional[str] = None
    image: Optional[str] = None
    logo: Optional[str] = None
    streaming_url: Optional[AnyHttpUrl] = None
    competitions: List[str] = Field([])
    deleted: Optional[datetime] = None

    _normalize_country = validator('country', allow_reuse=True)(check_country)
    # TODO[pydantic]: The following keys were removed: `json_encoders`.
    # Check https://docs.pydantic.dev/dev-v2/migration/#changes-to-config for more information.
    model_config = ConfigDict(populate_by_name=True, arbitrary_types_allowed=True, json_encoders={ObjectId: str}, json_schema_extra={
        "example": {
            "name": "Kings of the box",
            "code": "kob-2023",
            "start_date": "2023-06-16",
            "end_date": "2023-07-18",
            "country": "fra",
            "location": "allevard",
        }
    })

    async def check(self):
        for competition in self.competitions:
            if competition == "":
                raise HTTPException(400, "season code must be a non empty string")
            try:
                await Competition.get(competition)
            except:
                raise HTTPException(400, f"Competition '{competition}' not found")

    async def create(self):
        await self.check()
        try:
            event = jsonable_encoder(self)
            event['deleted'] = None
            res = await collection.insert_one(event)
            self.id = res.inserted_id
            return self
        except pymongo.errors.DuplicateKeyError:
            raise HTTPException(400, f"Event '{self.name}' already exists")

    async def save(self):
        await self.check()
        event = jsonable_encoder(self)

        old = await collection.find_one({"_id": str(self.id)})
        if old and event == old:
            return

        res =  await collection.update_one({"_id": str(self.id)}, {"$set": event})
        if res.modified_count != 1:
            raise HTTPException(400, f"Error while saving event {self.id}, 1 item should have been saved, got {res.modified_count}")

    async def export(self, cache:Cache = None) -> EventExport:

        competitions = []
        for competition in self.competitions:
            competition = await Competition.get(competition)
            competition = await competition.export_public(cache=cache)
            competitions.append(competition)

        event = EventExport(
            _id = str(self.id),
            name = self.name,
            code = self.code,
            start_date = self.start_date,
            end_date = self.end_date,
            country = self.country,
            location = self.location,
            website = self.website,
            description = self.description,
            image_url = f"/files/{self.image}" if self.image is not None else None,
            logo_url = f"/files/{self.image}" if self.logo is not None else None,
            streaming_url = self.streaming_url,
            competitions = competitions,
        )
        return event

    @staticmethod
    def createIndexes():
        collection.create_index([('name', pymongo.ASCENDING), ('deleted', pymongo.ASCENDING)], unique=True)
        collection.create_index([('code', pymongo.ASCENDING), ('deleted', pymongo.ASCENDING)], unique=True)
        log.debug('index created on "name,deleted" and "code,deleted"')

    @staticmethod
    async def get(id, deleted: bool = False, cache:Cache = None):
        if id is None:
            raise HTTPException(404, f"Event not found")

        if not deleted and cache is not None:
            event = cache.get('events', id)
            if event is not None:
                return event

        search = {"$or": [{"_id": id}, {"code": id}]}
        if deleted:
            search['id'] = "deleted"

        event = await collection.find_one(search)
        if event is None:
            raise HTTPException(404, f"Event {id} not found")

        event = Event.model_validate(event)
        if not deleted and cache is not None:
            cache.add('events', event)
        return event

    @staticmethod
    async def getall(deleted: bool = False, cache:Cache = None):
        if deleted:
            search = {}
        else:
            search = {"deleted": None}
            if cache is not None:
                events = cache.get_all('events')
                if events is not None:
                    return events
        events = []
        for event in await collection.find(search, sort=[("code", pymongo.DESCENDING), ("name", pymongo.ASCENDING)]).to_list(1000):
            event = Event.model_validate(event)
            events.append(event)
            if not deleted and cache is not None:
                cache.add('events', event)
        if not deleted and cache is not None:
            cache.set_all('events', events)
        return events

    @staticmethod
    async def update(id: str, event_update):
        event = await Event.get(id)
        event_update.id = event.id
        await event_update.save()

    @staticmethod
    async def delete(id: str, restore: bool = False):
        event = await Event.get(id, True)

        if restore ^ (event.deleted is not None):
            if restore:
                raise HTTPException(400, f"Can't restore Event {id} as it's not deleted")
            else:
                raise HTTPException(400, f"Can't delete Event {id} as it's already deleted")

        if restore:
            event.deleted = None
        else:
            event.deleted = datetime.now()
        await event.save()
