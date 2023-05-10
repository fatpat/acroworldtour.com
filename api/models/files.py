import logging
import sys
import hashlib
from pydantic import BaseModel, Field, validator, root_validator, PositiveInt
from bson import ObjectId
from typing import List, Optional
from fastapi.encoders import jsonable_encoder
import pymongo
from datetime import datetime
from fastapi import HTTPException

from core.config import settings

from core.database import db, PyObjectId
log = logging.getLogger(__name__)
collection = db.files

class FileID(BaseModel):
    id: str

class File(BaseModel):
    id: Optional[str] = Field(alias="_id")
    filename: str = Field(..., min_length=1)
    content_type: str = Field(..., min_length=1)
    content: str = Field(..., min_length=1)
    deleted: Optional[datetime]

    class Config:
        allow_population_by_field_name = True
        arbitrary_types_allowed = True
        json_encoders = {ObjectId: str}
        schema_extra = {
            "example": {
                "filename": "file.png",
                "content_type": "image/jpg",
                "size": 2,
                "content": "\x42\x54"
            }
        }

    async def create(self):
        try:
            self.id = hashlib.sha256(self.content.encode()).hexdigest()
            file = jsonable_encoder(self)
            file['deleted'] = None
            res = await collection.insert_one(file)
        except pymongo.errors.DuplicateKeyError:
            await self.save()

        return self

    async def save(self):
        file = jsonable_encoder(self)

        old = await collection.find_one({"_id": str(self.id)})
        if old and file == old:
            return

        res =  await collection.update_one({"_id": str(self.id)}, {"$set": file})
        if res.modified_count != 1:
            raise HTTPException(400, f"Error while saving file {self.id}, 1 item should have been saved, got {res.modified_count}")

    @staticmethod
    async def get(id, deleted: bool = False, cache:dict = {}):
        if id is None:
            raise HTTPException(404, f"File not found")

        if not deleted and 'files' in cache:
            try:
                return [j for j in cache['files'] if str(j.id) == id][0]
            except:
                pass

        if deleted:
            search = {"_id": id}
        else:
            search = {"_id": id, "deleted": None}
        file = await collection.find_one(search)
        if file is None:
            raise HTTPException(404, f"File {id} not found")
        return File.parse_obj(file)

    @staticmethod
    async def getall(deleted: bool = False):
        if deleted:
            search = {}
        else:
            search = {"deleted": None}
        files = []
        for file in await collection.find(search, sort=[("level", pymongo.DESCENDING), ("name", pymongo.ASCENDING)]).to_list(1000):
            files.append(File.parse_obj(file))
        return files

    @staticmethod
    async def update(id: str, file_update):
        file = await File.get(id)
        file_update.id = file.id
        await file_update.save()

    @staticmethod
    async def delete(id: str, restore: bool = False):
        file = await File.get(id, True)

        if restore ^ (file.deleted is not None):
            if restore:
                raise HTTPException(400, f"Can't restore File {id} as it's not deleted")
            else:
                raise HTTPException(400, f"Can't delete File {id} as it's already deleted")

        if restore:
            file.deleted = None
        else:
            file.deleted = datetime.now()
        await file.save()
