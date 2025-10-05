import logging
import core.logging
from fastapi import FastAPI, Request, status
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from contextlib import asynccontextmanager
import time

from redis import asyncio as aioredis

from core.config import settings
from core.database import clean_database
from routers.api import router
from controllers.judges import JudgeCtrl
from controllers.pilots import PilotCtrl
from controllers.teams import TeamCtrl
from controllers.tricks import TrickCtrl
from controllers.competitions import CompCtrl
from controllers.seasons import SeasonCtrl

log = logging.getLogger(__name__)

@asynccontextmanager
async def lifespan(app: FastAPI):
    if "test" in settings.DATABASE:
        log.debug(f"Using a testing database")
        await clean_database()

    JudgeCtrl.start()
    PilotCtrl.start()
    TeamCtrl.start()
    TrickCtrl.start()
    CompCtrl.start()
    SeasonCtrl.start()
    yield

app = FastAPI(
    title=settings.PROJECT_NAME,
    description=settings.PROJECT_DESCRIPTION,
    version=settings.VERSION,
    contact={
        "name": "Jérôme Loyet",
        "url": "https://github.com/fatpat",
        "email": "jerome@loyety.net",
    },
    license_info= {
        "name": "WTFPL 2",
        "url": "http://www.wtfpl.net/txt/copying/"
    },
    lifespan=lifespan,
)

@app.middleware('http')
async def validate_ip(request: Request, call_next):
    # Get client IP
    ip = str(request.client.host)
                
    # Check if IP is allowed
    if ip not in [""]:
        return JSONResponse(status_code=status.HTTP_403_FORBIDDEN, content="/dev/null")

    # Proceed if IP is allowed
    return await call_next(request)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
    expose_headers=['Content-Disposition'],
)

# add a X-Process-Time header
@app.middleware("http")
async def add_process_time_header(request: Request, call_next):
    start_time = time.time()
    response = await call_next(request)
    process_time = time.time() - start_time
    response.headers["X-Process-Time"] = str(process_time)
    return response

app.include_router(router)
