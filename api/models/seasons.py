import logging
from pydantic import BaseModel, Field, validator, AnyHttpUrl
from bson import ObjectId
from enum import Enum
from pycountry import countries
from typing import Optional, List
from fastapi.encoders import jsonable_encoder
import pymongo
from datetime import datetime
from fastapi import HTTPException

from models.competitions import Competition, CompetitionPublicExportWithResults, CompetitionState, CompetitionType
from models.pilots import Pilot
from models.teams import Team, TeamExport

from core.database import db, PyObjectId
from core.config import settings

log = logging.getLogger(__name__)
collection = db.seasons

class SeasonResult(BaseModel):
    pilot: Optional[Pilot]
    team: Optional[TeamExport]
    score: float

    class Config:
        allow_population_by_field_name = True
        arbitrary_types_allowed = True
        json_encoders = {ObjectId: str}

class SeasonResults(BaseModel):
    type: str
    results: List[SeasonResult]

    class Config:
        allow_population_by_field_name = True
        arbitrary_types_allowed = True
        json_encoders = {ObjectId: str}


class SeasonExport(BaseModel):
    id: str = Field(alias="_id")
    name: str
    code: str
    year: int
    image: Optional[AnyHttpUrl]
    type: CompetitionType
    number_of_pilots: int
    number_of_teams: int
    competitions: List[CompetitionPublicExportWithResults]
    results: List[SeasonResults]


    class Config:
        allow_population_by_field_name = True
        arbitrary_types_allowed = True
        json_encoders = {ObjectId: str}

class Season(BaseModel):
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    name: str = Field(..., description="The name of the season", min_length=2)
    code: str = Field(description="The short code of the season", min_length=2)
    year: int = Field(..., description="The year of the season", gt=1900)
    image: Optional[str]
    image_url: Optional[AnyHttpUrl]
    deleted: Optional[datetime]


    class Config:
        allow_population_by_field_name = True
        arbitrary_types_allowed = True
        json_encoders = {ObjectId: str}
        schema_extra = {
            "example": {
                "name": "Acro World Tour 2022",
                "code": "awt-2022"
            }
        }

    async def create(self):
        try:
            season = jsonable_encoder(self)
            season['deleted'] = None
            res = await collection.insert_one(season)
            self.id = res.inserted_id
            return self
        except pymongo.errors.DuplicateKeyError:
            raise HTTPException(400, f"Season '{self.name}' already exists")

    async def save(self):
        season = jsonable_encoder(self)

        old = await collection.find_one({"_id": str(self.id)})
        if old and season == old:
            return

        res =  await collection.update_one({"_id": str(self.id)}, {"$set": season})
        if res.modified_count != 1:
            raise HTTPException(400, f"Error while saving season {self.id}, 1 item should have been saved, got {res.modified_count}")

    @staticmethod
    def createIndexes():
        collection.create_index([('name', pymongo.ASCENDING), ('deleted', pymongo.ASCENDING)], unique=True)
        collection.create_index([('code', pymongo.ASCENDING), ('deleted', pymongo.ASCENDING)], unique=True)
        log.debug('index created on "name,deleted" and on "code,deleted"')

    @staticmethod
    async def get(id, deleted: bool = False, cache:dict = {}):
        if id is None:
            raise HTTPException(404, f"Season not found")

        if not deleted and 'seasons' in cache:
            try:
                return [j for j in cache['seasons'] if str(j.id) == id or j.code == id][0]
            except:
                pass

        search = {"$or": [{"_id": id}, {"code": id}]}
        if not deleted:
            search['deleted'] = None

        season = await collection.find_one(search)
        if season is None:
            raise HTTPException(404, f"Season {id} not found")
        season = Season.parse_obj(season)
        season.image_url = season.get_image_url()
        return season

    @staticmethod
    async def getall(deleted: bool = False):
        if deleted:
            search = {}
        else:
            search = {"deleted": None}
        seasons = []
        for season in await collection.find(search, sort=[("level", pymongo.DESCENDING), ("name", pymongo.ASCENDING)]).to_list(1000):
            season = Season.parse_obj(season)
            season.image_url = season.get_image_url()
            seasons.append(season)
        return seasons

    @staticmethod
    async def update(id: str, season_update):
        season = await Season.get(id)
        season_update.id = season.id
        if season_update.image is not None:
            season_update.image = season_update.image.split('/')[-1]

        await season_update.save()

    @staticmethod
    async def delete(id: str, restore: bool = False):
        season = await Season.get(id, True)

        if restore ^ (season.deleted is not None):
            if restore:
                raise HTTPException(400, f"Can't restore Season {id} as it's not deleted")
            else:
                raise HTTPException(400, f"Can't delete Season {id} as it's already deleted")

        if restore:
            season.deleted = None
        else:
            season.deleted = datetime.now()
        await season.save()

    def get_image_url(self):
        if self.image is not None:
            return f"{settings.SERVER_HOST}/files/{self.image}"
        return None

    async def export(self, cache:dict={}) -> SeasonExport:
        competitions = []
        results = {}
        _type = None
        teams = {}
        pilots = {}

        for comp in await Competition.getall(season=self.code):
            comp = await comp.export_public_with_results(cache=cache)

            # handle type and check that all comp of the season are of the same type
            if _type is None:
                _type = comp.type
            elif comp.type != _type:
                raise HTTPException(500, f"All competition of a season must be of the same type (either solo or synchro). Both are link to the season {self.name}")

            # add the comp to the list of comps
            competitions.append(comp)

            # only count published and closed competitions
            if comp.published and comp.state == CompetitionState.closed and comp.results.final:
                for res in comp.results.overall_results:
                    if _type == CompetitionType.synchro:
                        pilot_or_team = res.team.id
                        teams[res.team.id] = 0
                        for pilot in res.team.pilots:
                            pilots[pilot.civlid] = 0
                    else:
                        pilot_or_team = res.pilot.civlid
                        pilots[res.pilot.civlid] = 0
                    if pilot_or_team not in results:
                        results[pilot_or_team] = 0
                    results[pilot_or_team] += res.score

        overall = SeasonResults(type="overall", results=[])
        for pilot_or_team, score in results.items():
            if _type == CompetitionType.solo:
                overall.results.append(SeasonResult(
                    pilot = await Pilot.get(pilot_or_team, cache=cache),
                    score = score,
                ))
            if _type == CompetitionType.synchro:
                team = await Team.get(pilot_or_team, cache=cache)
                overall.results.append(SeasonResult(
                    team = await team.export(cache=cache),
                    score = score,
                ))
        overall.results.sort(key=lambda r:r.score, reverse=True)

        results = []
        results.append(overall)

        return SeasonExport(
            _id = str(self.id),
            name = self.name,
            code = self.code,
            year = self.year,
            image = self.get_image_url(),
            type = _type or CompetitionType.solo,
            number_of_pilots = len(pilots.keys()),
            number_of_teams = len(teams.keys()),
            competitions = competitions,
            results = results,
        )
