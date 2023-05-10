import logging
from pydantic import BaseModel, Field, validator, HttpUrl
from bson import ObjectId
from enum import Enum
from pycountry import countries
from typing import Optional, List
from datetime import datetime
from fastapi.encoders import jsonable_encoder
import pymongo
from fastapi import HTTPException

from core.database import db, PyObjectId

log = logging.getLogger(__name__)
collection = db.pilots

class Link(BaseModel):
    name: str
    link: HttpUrl

class Sponsor(BaseModel):
    name: str
    link: HttpUrl
    img: str

class GenderEnum(str, Enum):
    man   = 'man'
    woman = 'woman'
    none  = 'none'


class Pilot(BaseModel):
    id: int = Field(..., alias="_id")
    civlid: int = Field(..., description="The CIVL ID of the pilot")
    name: str = Field(..., description="The complete name of the pilot")
    link: HttpUrl = Field(..., description="The link to the CIVL pilot page")
    country: str = Field(..., description="The country of the pilot")
    about: str = Field(..., description="About text of the pilot")
    links: List[Link] = Field(..., description="List of pilot's links (socials medias, ...)")
    sponsors: List[Sponsor] = Field(..., description="List of the pilot's sponsors")
    photo: HttpUrl = Field(..., description="Link to the profile image of the pilot")
    background_picture: HttpUrl = Field(..., description="Link to the background profile image of the pilot")
    last_update: Optional[datetime] = Field(None, description="Last time the pilot has been updated")
    rank: int = Field(..., description="Current pilot's ranking in the aerobatic solo overwall world ranking")
    gender: GenderEnum = Field(GenderEnum.man, description="Pilot's sex")

    class Config:
        allow_population_by_field_name = True
        arbitrary_types_allowed = True
        json_encoders = {ObjectId: str}
        schema_extra = {
            "example": {
                "civlid": 67619,
                "name": "Luke de Weert",
                "link": "https://civlcomps.org/pilot/67619",
                "country": "nld",
                "about": "\"I am an athlete who believes that dedication is the core of the thing that keeps me pushing and motivating me to achieve all my goals, and even set new goals where I never thought it was possible.\"",
                "links": [
                    {"name": "facebook", "link": "https://www.facebook.com/deweert.luke"},
                    {"name": "instagram", "link": "https://www.instagram.com/luke_deweert/"},
                    {"name": "twitter", "link": "https://twitter.com/luke_deweert"},
                    {"name": "youtube", "link": "https://www.youtube.com/lukedeweert"},
                    {"name": "Website", "link": "https://lukedeweert.nl"},
                    {"name": "Tiktok",  "link": "https://www.tiktok.com/@lukedeweert"}
                ],
                "sponsors": [
                    {"name": "Sky Paragliders", "link": "https://sky-cz.com/en", "img": "https://civlcomps.org/uploads/images/ems_event_sponsor_logo/1/4cbe1ebac175a9cde7a4c9d8769ba0c4/509e4e83c097d02828403b5a67e8c0b5.png"},
                    {"name": "Sinner", "link": "https://www.sinner.eu/nl/", "img": "https://civlcomps.org/uploads/images/ems_event_sponsor_logo/1/dddccfa819ee01d9b2410ba49fa432fc/eeff42d05ffefb8ef945dc83485007ea.png"},
                    {"name": "Wanbound", "link": "https://www.wanbound.com/", "img": "https://civlcomps.org/uploads/images/ems_event_sponsor_logo/1/aa675f347b7d7933332df96f08b21199/4ff22ae0404446f203ba682751e1e7b8.png"},
                    {"name": "KNVvL","link": "https://www.knvvl.nl/", "img": "https://civlcomps.org/uploads/images/ems_event_sponsor_logo/1/53ee05f2c2172541b7f1dd99e67a59f9/0f68789e476c0494019a750a6da9c6aa.png"}
                ],
                "photo": "https://civlcomps.org/uploads/images/profile/676/7bdecbee5d2246b1ebc14248dc1af935/8bfbe7e62a481a19145c55c9dc97e6ab.jpeg",
                "background_picture": "https://civlcomps.org/uploads/images/pilot_header/9/c017697641aa9ef817c4c17728e9e6d6/08788da048eea61f93be8591e97f6a0c.jpg",
                "last_update": "2022-06-03T19:05:59.325692",
                "rank": 2
            }
        }

    async def save(self):
        self.last_update = datetime.now()
        pilot = jsonable_encoder(self)
        try:
            await collection.insert_one(pilot)
        except pymongo.errors.DuplicateKeyError:
            await collection.update_one({"_id": self.id}, {"$set": pilot})

        return self

    def change_gender(self):
        if self.gender == GenderEnum.man:
            self.gender = GenderEnum.woman
        elif self.gender == GenderEnum.woman:
            self.gender = GenderEnum.man

    @staticmethod
    async def get(id: int, cache:dict = {}):
        if 'pilots' in cache:
            try:
                return [p for p in cache['pilots'] if p.id == id][0]
            except:
                pass

        pilot = await collection.find_one({"_id": id})

        if pilot is None:
            raise HTTPException(status_code=404, detail=f"Pilot {id} not found")

        return Pilot.parse_obj(pilot)

    @staticmethod
    async def getall(list:List[str] = []):
        if len(list) > 0:
            cond = {"$or": [
                {"id": {"$in": list}},
                {"name": {"$in": list}}
            ]}
        else:
            cond = {}
        pilots = []
        sort=[("rank", pymongo.ASCENDING),("name", pymongo.ASCENDING)]
        for pilot in await collection.find(filter=cond, sort=sort).to_list(1000):
            pilots.append(Pilot.parse_obj(pilot))
        return pilots

    @staticmethod
    async def delete(id: int):
        pilot = await Pilot.get(id)
        return await collection.delete_one({"_id": id})

    @staticmethod
    def createIndexes():
        collection.create_index('civlid', unique=True)
        collection.create_index('name', unique=True)
        log.debug('indexes created on "civlid" and "name"')
