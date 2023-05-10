import logging
from pydantic import BaseModel, Field, validator
from bson import ObjectId
from typing import List, Optional
from fastapi.encoders import jsonable_encoder
from datetime import datetime
import pymongo
from fastapi import HTTPException

from models.pilots import Pilot

from core.database import db, PyObjectId
log = logging.getLogger(__name__)
collection = db.teams

class TeamExport(BaseModel):
    id: str = Field(alias="_id")
    name: str
    pilots: List[Pilot]

class Team(BaseModel):
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    name: str = Field(..., description="The name of the team")
    pilots: List[int] = Field(..., description="The 2 pilots composing the team (by CIVLID)")
    deleted: Optional[datetime]

    @validator('pilots')
    def check_pilots(cls, v):
        if len(v) != 2:
            raise ValueError('2 pilots must compose a team')
        return v

    class Config:
        allow_population_by_field_name = True
        arbitrary_types_allowed = True
        json_encoders = {ObjectId: str}
        schema_extra = {
            "example": {
                "_id": "687687687687aze",
                "name": "Team Rocket",
                "pilots": [1234, 4567]
            }
        }

    async def check(self):
        pilots = []
        for id in self.pilots:
            pilot = await Pilot.get(id)
            if pilot is None:
                raise HTTPException(400, f"Pilot '{id}' is unknown, only known pilots can be part of a team")
            pilots.append(pilot.civlid)
        self.pilots=pilots

    async def create(self):
        await self.check()
        try:
            team = jsonable_encoder(self)
            team['deleted'] = None
            res = await collection.insert_one(team)
            self.id = res.inserted_id
            return self
        except pymongo.errors.DuplicateKeyError:
            raise HTTPException(400, f"Team '{self.name}' already exists")

    async def save(self):
        await self.check()
        team = jsonable_encoder(self)

        old = await collection.find_one({"_id": str(self.id)})
        if old and team == old:
            return

        res =  await collection.update_one({"_id": str(self.id)}, {"$set": team})
        if res.modified_count != 1:
            raise HTTPException(400, f"Error while saving team, 1 item should have been saved, got {res.modified_count}")

    async def export(self, cache:dict = {}) -> TeamExport:
        pilots = []
        for pilot in self.pilots:
            pilots.append(await Pilot.get(pilot, cache=cache))

        return TeamExport(
                _id=str(self.id),
                name=self.name,
                pilots=pilots,
        )

    @staticmethod
    def createIndexes():
        collection.create_index([('name', pymongo.ASCENDING), ('deleted', pymongo.ASCENDING)], unique=True)
        log.debug('index created on "name,deleted"')

    @staticmethod
    async def get(id, deleted: bool = False, cache:dict = {}):
        if id is None or id == '':
            raise HTTPException(404, f"Team not found")

        if not deleted and 'teams' in cache:
            try:
                return [t for t in cache['teams'] if str(t.id) == id][0]
            except:
                pass

        if deleted:
            search = {"_id": id}
        else:
            search = {"_id": id, "deleted": None}
        team = await collection.find_one(search)
        if team is None:
            raise HTTPException(404, f"Team {id} not found")
        return Team.parse_obj(team)

    @staticmethod
    async def getall(deleted: bool = False):
        if deleted:
            search = {}
        else:
            search = {"deleted": None}
        teams = []
        for team in await collection.find(search, sort=[("name", pymongo.ASCENDING)]).to_list(1000):
            teams.append(Team.parse_obj(team))
        return teams

    @staticmethod
    async def update(id: str, team_update):
        team = await Team.get(id)
        if team is None:
            return None

        team_update.id = team.id
        team_update.deleted = team.deleted
        return await team_update.save()

    @staticmethod
    async def delete(id: str, restore: bool = False):
        team = await Team.get(id, True)

        if restore ^ (team.deleted is not None):
            if restore:
                raise HTTPException(400, f"Can't restore Team {id} as it's not deleted")
            else:
                raise HTTPException(400, f"Can't delete Team {id} as it's already deleted")

        if restore:
            team.deleted = None
        else:
            team.deleted = datetime.now()
        await team.save()
