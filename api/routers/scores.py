import logging
from fastapi import APIRouter, Depends, Body, HTTPException
from core.security import auth
from core.config import settings

from controllers.scores import ScoreCtrl
from models.flights import Flight, FlightNew
from models.marks import FinalMark
from models.competitions import CompetitionType

log = logging.getLogger(__name__)
scores= APIRouter()

@scores.post(
    "/simulate/{t}",
    response_model=FinalMark,
    dependencies=[Depends(auth)]
)
async def simulate(t: CompetitionType, flight: FlightNew = Body(...)):
    return await ScoreCtrl.simulate_score(flight, t)
