import logging
import sys
from pydantic import field_validator, ConfigDict, BaseModel, Field, root_validator, AnyHttpUrl
from bson import ObjectId
from typing import List, Optional
from fastapi.encoders import jsonable_encoder
import pymongo
from datetime import datetime
from fastapi import HTTPException

from core.config import settings

from core.database import db, PyObjectId
from models.cache import Cache
log = logging.getLogger(__name__)
collection = db.tricks

class Bonus(BaseModel):
    name: str = Field(..., min_length=1)
    bonus: float = Field(..., ge=0.0)
    sample_video: Optional[AnyHttpUrl] = None
    model_config = ConfigDict(json_schema_extra={
        "example": {
            "name": "twisted",
            "bonus": 2.5
        }
    })
    @field_validator('name')
    @classmethod
    def check_name(cls, v):
        bonuses = list(map(lambda x: x['name'], settings.tricks.available_bonuses))
        if v not in bonuses:
            bonuses = ", ".join(bonuses)
            raise ValueError(f"Invalid bonus ({v}), must be one of: {bonuses}")
        return v


class UniqueTrick(BaseModel):
    name: str
    acronym: str
    technical_coefficient: float
    bonus: float
    bonus_types: List[str]
    base_trick: str
    uniqueness: List[str]
    bonuses: List[Bonus] = []
    synchro: Optional[bool] = None
    solo: Optional[bool] = None
    solo_awt: Optional[bool] = None
    types: List[str] = []
    technical_mark: Optional[float] = None
    technical_marks: Optional[dict[str, float]] = None
    model_config = ConfigDict(json_schema_extra={
        "example": {
            "name": "twisted left Misty to Helicopter reverse",
            "acronym": "/LMHR",
            "technical_coefficient": 1.75,
            "bonus": 6,
            "bonus_types": ["twist", "reverse"],
            "uniqueness": ["left", "reverse"],
            "base_trick": "Misty To Helicoper",
        }
    })

class Trick(BaseModel):
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    name: str = Field(..., min_length=1, description="The name of the trick (without bonuses)")
    acronym: str = Field(..., min_length=1, description="The acronym of the trick (without bonuses)")
    types: List[str] = Field([], description="List of trick types")
    solo: bool = Field(..., description="Is this trick valid for solo competitions")
    solo_awt: bool = Field(True, description="Is this trick valid for solo competitions")
    synchro: bool = Field(..., description="Is this trick valid for synchro competitions")
    directions: List[str] = Field(..., description="List of allowed diredctions for the trick. Empty list implies a trick with a unique direction")
    technical_coefficient: float = Field(..., ge=0.0, description="The technical coefficient of the trick")
    bonuses: List[Bonus] = Field(..., description="List of all bonuses that can apply to this trick")
    bonus_constraints: List[List[str]] = Field([], description="List of bonuses that are exclusive to each other")
    tricks_constraints: List[str] = Field([], description="List of tricks that cannot be performed during the same competition.")
    first_maneuver: int = Field(0, ge=0, description="If positive, indicates that the trick must be performed in the first N tricks of the run")
    no_first_maneuver: int = Field(0, ge=0, description="If positive, indicates that the trick must not be performed in the first N tricks of the run")
    last_maneuver: int = Field(0, ge=0, description="If positive, indicates that the trick must be performed in the last N tricks of the run")
    no_last_maneuver: int = Field(0, ge=0, description="If positive, indicates that the trick must not be performed in the last N tricks of the run")
    tricks: List[UniqueTrick] = Field([], description="List of all the variant of the trick (this is automatically generated)")
    repeatable: bool = Field(False, description="Is this trick can be repeatable")
    deleted: Optional[datetime] = None
    sample_video: Optional[AnyHttpUrl] = None

    @field_validator('directions')
    @classmethod
    def check_directions(cls, v):
        if len(v) == 1:
            raise ValueError("directions must have 0 or at least 2 directions (not one)")

        if len(v) > 1:
            if len(v) != len(list(set(v))): # using set() to get unique values of directions
                raise ValueError("can have a same direction declared more than once")

        for direction in v:
            directions = list(map(lambda  x:x['name'], settings.tricks.available_directions))
            if direction not in directions:
                directions = ", ".join(directions)
                raise ValueError(f"invalid direction '{direction}', must be one of {directions} ")
        return v

    @field_validator('bonus_constraints')
    @classmethod
    def check_bonus_constraints(cls, v):
        for constraints in v:
            if len(constraints) != 2:
                raise ValueError(f"invalid bonus contraints '{contraints}', must be composed of strictly 2 bonuses")
        return v
    # TODO[pydantic]: The following keys were removed: `json_encoders`.
    # Check https://docs.pydantic.dev/dev-v2/migration/#changes-to-config for more information.
    model_config = ConfigDict(populate_by_name=True, arbitrary_types_allowed=True, json_encoders={ObjectId: str}, json_schema_extra={
        "example": {
            "_id": "bababababaabababababab",
            "name": "Misty to Helicopter",
            "acronym": "MH",
            "solo": True,
            "synchro": True,
            "directions": ["left", "right"],
            "technical_coefficient": 1.75,
            "bonuses": [
                {"name": "twisted", "bonus": 3},
                {"name": "reverse", "bonus": 3}
            ],
            "first_maneuver": 0,
            "no_first_maneuver": 0, 
            "last_maneuver": 0,
            "no_last_maneuver": 0,
            "repeatable": False,
            "tricks": [
                {      "name": "left Misty to Helicopter",      "acronym": "LMH",      "technical_coefficient": 1.75,      "bonus": 0    },
                {      "name": "right Misty to Helicopter",      "acronym": "RMH",      "technical_coefficient": 1.75,      "bonus": 0    },
                {      "name": "twisted left Misty to Helicopter",      "acronym": "/LMH",      "technical_coefficient": 1.75,      "bonus": 3    },
                {      "name": "twisted right Misty to Helicopter",      "acronym": "/RMH",      "technical_coefficient": 1.75,      "bonus": 3    },
                {      "name": "left Misty to Helicopter reverse",      "acronym": "LMHR",      "technical_coefficient": 1.75,      "bonus": 3    },
                {      "name": "right Misty to Helicopter reverse",      "acronym": "RMHR",      "technical_coefficient": 1.75,      "bonus": 3    },
                {      "name": "twisted left Misty to Helicopter reverse",      "acronym": "/LMHR",      "technical_coefficient": 1.75,      "bonus": 6    },
                {      "name": "twisted right Misty to Helicopter reverse",      "acronym": "/RMHR",      "technical_coefficient": 1.75,      "bonus": 6    }
            ]
        }
    })

    async def check(self):
#        for id in self.pilots:
#            pilot = await Pilot.get(id)
#            if pilot is None:
#                raise HTTPException(400, f"Pilot '{id}' is unknown, only known pilots can be part of a trick")
        return

    async def create(self):
        try:
            self.deleted = None
            await self.check()
            trick = jsonable_encoder(self)
            res = await collection.insert_one(trick)
            self.id = res.inserted_id
            return self
        except pymongo.errors.DuplicateKeyError:
            raise HTTPException(400, f"Trick '{self.name}' already exists")

    async def save(self):
        await self.check()
        trick = jsonable_encoder(self)

        old = await collection.find_one({"_id": str(self.id)})
        if old and trick == old:
            return

        res = await collection.update_one({"_id": str(self.id)}, {"$set": trick})
        if res.modified_count != 1:
            raise HTTPException(400, f"Error while saving Trick {self.id}, 1 item should have been saved, got {res.modified_count}")

    @staticmethod
    def createIndexes():
        collection.create_index([('name', pymongo.ASCENDING), ('deleted', pymongo.ASCENDING)], unique=True)
        collection.create_index([('acronym', pymongo.ASCENDING), ('deleted', pymongo.ASCENDING)], unique=True)
        log.debug('index created on "name,deleted" and "acronym,deleted"')

    @staticmethod
    async def get(id, deleted: bool = False, cache:Cache = None):
        if id is None:
            raise HTTPException(404, f"Trick not found")

        if not deleted and cache is not None:
            trick = cache.get('tricks', id)
            if trick is not None:
                return trick

        if deleted:
            search = {"$or": [{"_id": id}, {"name": id}]}
        else:
            search = {"$or": [{"_id": id}, {"name": id}], "deleted": None}
        trick = await collection.find_one(search)
        if trick is None:
            raise HTTPException(404, f"Trick {id} not found")

        if 'bonus_constraints' not in trick:
            trick['bonus_constraints'] = []

        trick = Trick.model_validate(trick)
        if not deleted and cache is not None:
            cache.add('tricks', trick)

        return trick

    @staticmethod
    async def get_unique_tricks(solo: bool, synchro: bool) -> List[UniqueTrick]:
        log.debug(f"get_unique_tricks solo={solo} synchro={synchro}")
        if not solo and not synchro:
            return []
        tricks = []
        for trick in await collection.find({"deleted": None, "$or": [{"solo": solo}, {"synchro": synchro}]}).sort("technical_coefficient").to_list(1000):
            for t in trick['tricks']:
                if ('solo' not in t or t['solo'] == solo) or ('synchro' not in t or t['synchro'] == synchro):
                    tricks.append(UniqueTrick.model_validate(t))
        return tricks

    @staticmethod
    async def get_unique_trick(id) -> UniqueTrick:
        trick = await collection.find_one({"deleted": None, "$or": [{"tricks.name": id}, {"tricks.acronym": id}]})
        if trick is None:
            return None
        trick = Trick.model_validate(trick)
        for t in trick.tricks:
            if t.name == id or t.acronym == id:
                return UniqueTrick.model_validate(t.dict())
        return None

    @staticmethod
    async def getall(deleted: bool = False, repeatable: bool = None, cache:Cache = None):
        tricks = []
        if deleted:
            search = {}
        else:
            search = {"deleted": None}
            if cache is not None:
                tricks = cache.get_all('tricks')
                if tricks is not None:
                    return tricks

        if repeatable is not None:
            search["repeatable"] = repeatable

        tricks = []
        for trick in await collection.find(search, sort=[("technical_coefficient", pymongo.ASCENDING)]).to_list(1000):

            trick = Trick.model_validate(trick)
            tricks.append(trick)
            if not deleted and cache is not None:
                cache.add('tricks', trick)

        if not deleted and cache is not None:
            cache.set_all('tricks', tricks)
        return tricks

    @staticmethod
    async def update(id: str, trick_update):
        trick = await Trick.get(id)
        if trick is None:
            return None

        trick_update.id = trick.id
        await trick_update.save()

    @staticmethod
    async def delete(id: str, restore: bool = False):
        trick = await Trick.get(id, True)
        if trick is None:
            return None

        if restore ^ (trick.deleted is not None):
            if restore:
                raise HTTPException(400, f"Can't restore Trick {id} as it's not deleted")
            else:
                raise HTTPException(400, f"Can't delete Trick {id} as it's already deleted")

        if restore:
            trick.deleted = None
        else:
            trick.deleted = datetime.now()
        await trick.save()
