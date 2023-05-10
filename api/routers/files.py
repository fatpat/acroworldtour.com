import logging
import os
import base64
from http import HTTPStatus
from fastapi import APIRouter, Depends, HTTPException, BackgroundTasks, Response, Request, UploadFile
from fastapi.responses import FileResponse
from typing import List
from datetime import datetime

from core.security import auth
from core.database  import db
from controllers.utils import UtilsCtrl
from models.files import File, FileID

log = logging.getLogger(__name__)
files = APIRouter()

#
# GET
#
@files.get(
    "/{id}",
    response_description="",
    response_class=Response,
)
async def get_file(request: Request, id: str, download : bool = False):
    file = await File.get(id)
    headers = {}
#        "Content-Length": str(len(file.content))
#    }
    if download:
        headers["Content-Disposition"] = f"attachment; filename=\"{file.filename}\""
    else:
        headers["Content-Disposition"] = f"inline"

    return Response(content=base64.b64decode(file.content), media_type=file.content_type, headers=headers)

#
# POST
#
@files.post(
    "/new",
    response_description="",
    response_model=FileID,
    dependencies=[Depends(auth)],
)
async def post_file(file: UploadFile):
    if not file:
        raise HTTPException(status_code=400, detail="missing file")
    f = File(
        filename=file.filename,
        content_type=file.content_type,
        content=base64.b64encode(await file.read()),
    )
    f = await f.create()
    return FileID(id=f.id)

#
# DELETE
#
@files.delete(
    "/{id}",
    status_code=204,
    response_description="Delete a File",
    response_class=Response,
    dependencies=[Depends(auth)],
)
async def delete(id: str, restore: bool = False):
    res = await File.delete(id, restore)
