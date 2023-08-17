import logging
import httpx
from http import HTTPStatus
from openpyxl import load_workbook
from tempfile import NamedTemporaryFile
import lxml.html
import re
from fastapi import HTTPException
from fastapi.concurrency import run_in_threadpool
from random import shuffle
from typing import List
from datetime import date

from core.config import settings
from core.utils import average, weight_average
from models.flights import Flight, FlightNew
from models.marks import JudgeMark, FinalMark
from models.competitions import Competition, CompetitionNew, CompetitionType, CompetitionConfig, CompetitionState
from models.tricks import Trick
from models.judges import Judge
from models.runs import Run, RunState, RunRepetitionsResetPolicy

log = logging.getLogger(__name__)

class ScoreCtrl:
    @staticmethod
    async def simulate_score(flight: FlightNew, type: CompetitionType) -> FinalMark:
        log.debug(flight)

        tricks = []
        errors = []
        for trick_name in flight.tricks:
            trick = await Trick.get_unique_trick(trick_name,
                solo  = (type==CompetitionType.solo),
                synchro = (type==CompetitionType.synchro),
            )
            if trick is None:
                errors.append(trick_name)
            else:
                tricks.append(trick)

        if len(errors) > 0:
            errors = ", ".join(errors)
            raise HTTPException(status_code=400, detail=f"Some tricks are unknown for a {type} run: {errors}")

        f = Flight(
            pilot = 0,
            team = '',
            tricks=tricks,
            marks=flight.marks,
            did_not_start=flight.did_not_start,
            warnings=flight.warnings,
        )

        competition = Competition(
            name="Simulated comp",
            start_date = date.today(),
            end_date = date.today(),
            type = type,
            state = CompetitionState.init,
            config = CompetitionConfig(),
            repeatable_tricks = [str(trick.id) for trick in await Trick.getall(repeatable=True)],
            pilots = [],
            teams = [],
            judges = [],
            runs = [],
            deleted = None,
            location = 'nowhere',
            published = False,
        )

        return await competition.calculate_score(flight=f)

    @staticmethod
    async def simulate_scores(flights: List[FlightNew], type: CompetitionType, reset_repetitions_frequency:int = 0) -> FinalMark:

        pilot1 = -1000000
        pilot2= -1000001

        competition = Competition(
            name="Simulated comp",
            start_date = date.today(),
            end_date = date.today(),
            type = type,
            state = CompetitionState.open,
            config = CompetitionConfig(),
            repeatable_tricks = [str(trick.id) for trick in await Trick.getall(repeatable=True)],
            pilots = [pilot1, pilot2],
            teams = [],
            judges = ['simulator1', 'simulator2'],
            runs = [],
            deleted = None,
            location = 'nowhere',
            published = False,
        )

        for (i, flight) in enumerate(flights):
            tricks = []
            errors = []
            for trick_name in flight.tricks:
                trick = await Trick.get_unique_trick(trick_name)
                if trick is None:
                    errors.append(trick_name)
                else:
                    tricks.append(trick)

            if len(errors) > 0:
                errors = ", ".join(errors)
                raise HTTPException(status_code=400, detail=f"Some tricks are unknown for {type} run number {i+1}: {errors}")

            reset_policy = RunRepetitionsResetPolicy.none
            if i > 0 and reset_repetitions_frequency > 0 and i % reset_repetitions_frequency == 0:
                reset_policy = RunRepetitionsResetPolicy.all
            run = await competition.new_run(0, reset_policy, saveToDB=False)

            # force judge name
            for mark in flight.marks:
                mark.judge = "simulator"

            f = Flight(
                pilot = pilot1,
                team = '',
                tricks=tricks,
                marks=flight.marks,
                did_not_start=flight.did_not_start,
                warnings=flight.warnings,
            )
            mark = await competition.flight_save(run_i=i, id=pilot1, flight=flight, save=True, published=True, saveToDB=False)

        results =  await competition.results()
        output = []
        for result in results.runs_results:
            f = result.results['overall'][0]
            f.pilot = 0
            output.append(f)

        return output
