import logging
import os
from http import HTTPStatus
from fastapi import APIRouter, Depends, HTTPException, BackgroundTasks, Response
from fastapi.responses import FileResponse
from typing import List
from datetime import datetime

from core.security import auth
from core.database  import db
from controllers.utils import UtilsCtrl

log = logging.getLogger(__name__)
utils = APIRouter()

#
# DB backup
#
@utils.get(
    "/backup",
    response_description="backup all database",
    response_class=FileResponse,
    dependencies=[Depends(auth)],
)
async def backup(bg_tasks: BackgroundTasks):
    backup_file = await UtilsCtrl.backup()
    bg_tasks.add_task(os.remove, backup_file)
    filename=f"backup-{db.name}-{datetime.now().strftime('%Y-%m-%d-%H%M%S')}.tar.gz"
    return FileResponse(path=backup_file, filename=filename, background=bg_tasks) 

#
# DB backup
#
@utils.post(
    "/cleanup_pilots",
    status_code=204,
    response_description="backup all database",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def backup():
    await UtilsCtrl.delete_unused_pilots()
