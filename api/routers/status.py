from fastapi import APIRouter

from core.config import settings

from models.status import Status

status = APIRouter()

@status.get(
    "/",
    response_model=Status,
)
async def get():
    return Status(
        project= settings.PROJECT_NAME,
        version= settings.VERSION,
    )
