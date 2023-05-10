import logging
from pydantic import BaseModel, Field, validator
from bson import ObjectId
from enum import Enum
from pycountry import countries
from typing import Optional
from fastapi.encoders import jsonable_encoder
import pymongo
from datetime import datetime
from fastapi import HTTPException

from core.database import db, PyObjectId

from models.pilots import Pilot

log = logging.getLogger(__name__)
collection = db.judges

def check_country(cls, v):
    assert countries.get(alpha_3=v) is not None, f"Invalid country '{v}'"
    return v

class JudgeLevel(Enum):
    trainee = "trainee"
    certified = "certified"
    senior = "senior"

class Judge(BaseModel):
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    name: str = Field(..., description="The full name of the judge", min_length=2)
    country: str = Field(..., description="The country of the judge using the 3 letter acronym of the country")
    level: JudgeLevel = Field(..., description="The level of the judge")
    civlid: Optional[int] = Field(None, description="The CIVL ID if any (must be registered in the pilot database")
    deleted: Optional[datetime]

    _normalize_country = validator('country', allow_reuse=True)(check_country)

    class Config:
        allow_population_by_field_name = True
        arbitrary_types_allowed = True
        json_encoders = {ObjectId: str}
        schema_extra = {
            "example": {
                "name": "Jerry The Judge",
                "country": "fra",
                "level": "certified",
                "civlid": 1234,
            }
        }

    async def check(self):
        if self.civlid is not None:
            try:
                await Pilot.get(self.civlid)
            except:
                raise HTTPException(400, f"CIVL ID #{self.civlid} is not known in Pilot Database. First add it to the database from the pilots page.")

    async def create(self):
        await self.check()
        try:
            judge = jsonable_encoder(self)
            judge['deleted'] = None
            res = await collection.insert_one(judge)
            self.id = res.inserted_id
            return self
        except pymongo.errors.DuplicateKeyError:
            raise HTTPException(400, f"Judge '{self.name}' already exists")

    async def save(self):
        await self.check()
        judge = jsonable_encoder(self)

        old = await collection.find_one({"_id": str(self.id)})
        if old and judge == old:
            return

        res =  await collection.update_one({"_id": str(self.id)}, {"$set": judge})
        if res.modified_count != 1:
            raise HTTPException(400, f"Error while saving judge {self.id}, 1 item should have been saved, got {res.modified_count}")

    @staticmethod
    def createIndexes():
        collection.create_index([('name', pymongo.ASCENDING), ('deleted', pymongo.ASCENDING)], unique=True)
        log.debug('index created on "name,deleted"')

    @staticmethod
    async def get(id, deleted: bool = False, cache:dict = {}):
        if id is None:
            raise HTTPException(404, f"Judge not found")

        if not deleted and 'judges' in cache:
            try:
                return [j for j in cache['judges'] if str(j.id) == id][0]
            except:
                pass

        if deleted:
            search = {"_id": id}
        else:
            search = {"_id": id, "deleted": None}
        judge = await collection.find_one(search)
        if judge is None:
            raise HTTPException(404, f"Judge {id} not found")
        return Judge.parse_obj(judge)

    @staticmethod
    async def getall(deleted: bool = False):
        if deleted:
            search = {}
        else:
            search = {"deleted": None}
        judges = []
        for judge in await collection.find(search, sort=[("level", pymongo.DESCENDING), ("name", pymongo.ASCENDING)]).to_list(1000):
            judges.append(Judge.parse_obj(judge))
        return judges

    @staticmethod
    async def update(id: str, judge_update):
        judge = await Judge.get(id)
        judge_update.id = judge.id
        await judge_update.save()

    @staticmethod
    async def delete(id: str, restore: bool = False):
        judge = await Judge.get(id, True)

        if restore ^ (judge.deleted is not None):
            if restore:
                raise HTTPException(400, f"Can't restore Judge {id} as it's not deleted")
            else:
                raise HTTPException(400, f"Can't delete Judge {id} as it's already deleted")

        if restore:
            judge.deleted = None
        else:
            judge.deleted = datetime.now()
        await judge.save()
