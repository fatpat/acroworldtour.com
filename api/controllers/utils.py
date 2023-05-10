import logging
import httpx
from http import HTTPStatus
from openpyxl import load_workbook
from tempfile import NamedTemporaryFile
import lxml.html
import re
from fastapi.concurrency import run_in_threadpool
from fastapi import HTTPException
from random import shuffle
from bson.json_util import dumps
import tarfile
import os
import asyncio

from core.database import PyObjectId, db
from core.config import settings
from models.pilots import Pilot
from models.teams import Team
from models.judges import Judge
from models.tricks import Trick
from models.competitions import Competition
from controllers.pilots import PilotCtrl

log = logging.getLogger(__name__)

class UtilsCtrl:
    @staticmethod
    async def backup():
        ret = None
        with NamedTemporaryFile(suffix=".tar.gz", delete=False) as f:
            ret = f.name
            with tarfile.open(mode='w:gz', fileobj=f) as tar:
                for collection in await db.list_collection_names():
                    objects = []
                    for o in await db[collection].find().to_list(1000):
                        objects.append(o)
                    with NamedTemporaryFile(suffix=".json") as json:
                        json.write(dumps(objects).encode())
                        json.flush() # ensure everything is written to disk before calling tar.add
                        tar.add(json.name, arcname=f"{db.name}/{collection}.json")
        
        return ret

    @staticmethod
    async def delete_unused_pilots() -> bool:
        for pilot in await PilotCtrl.get_all_unused_pilots():
            log.info(f"deleting pilot {pilot}")
            await Pilot.delete(pilot)

    @staticmethod
    async def get_cache() -> dict:
        return{
            "pilots": await Pilot.getall(),
            "teams": await Team.getall(),
            "judges": await Judge.getall(),
            "tricks": await Trick.getall()
        }
