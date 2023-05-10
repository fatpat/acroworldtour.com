from fastapi import APIRouter
from fastapi.responses import HTMLResponse

from routers.auth import auth_router
from routers.status import status
from routers.pilots import pilots
from routers.judges import judges
from routers.teams import teams
from routers.tricks import tricks
from routers.competitions import competitions
from routers.scores import scores
from routers.public import public
from routers.utils import utils
from routers.files import files
from routers.seasons import seasons

from core.config import settings

router = APIRouter()
router.include_router(auth_router, prefix="/auth", tags=["auth"])
router.include_router(status, prefix="/status", tags=["status"])
router.include_router(pilots, prefix="/pilots", tags=["pilots"])
router.include_router(judges, prefix="/judges", tags=["judges"])
router.include_router(teams, prefix="/teams", tags=["teams"])
router.include_router(tricks, prefix="/tricks", tags=["tricks"])
router.include_router(competitions, prefix="/competitions", tags=["competitions"])
router.include_router(scores, prefix="/scores", tags=["scores"])
router.include_router(public, prefix="/public", tags=["public"])
router.include_router(utils, prefix="/utils", tags=["utils"])
router.include_router(files, prefix="/files", tags=["files"])
router.include_router(seasons, prefix="/seasons", tags=["seasons"])

@router.get(
    "/",
    status_code=200,
    response_description="home page",
    response_class=HTMLResponse,
)
async def home():
    return f"""
    <html>
        <head>
            <title>{settings.PROJECT_NAME} version {settings.VERSION}</title>
        </head>
        <body>
            <h1>{settings.PROJECT_NAME} version {settings.VERSION}</h1>
        </body>
    </html>
    """
