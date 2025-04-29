import logging
from collections import Counter
from pydantic import ConfigDict, BaseModel, Field, validator, AnyHttpUrl
from fastapi import  HTTPException
from bson import ObjectId
from typing import List, Optional
from fastapi.encoders import jsonable_encoder
import pymongo
from enum import Enum
from datetime import date, datetime
import re
import unicodedata

from models.pilots import Pilot
from models.teams import Team, TeamExport
from models.judges import Judge
from models.runs import Run, RunState, RunExport, RunRepetitionsResetPolicy
from models.tricks import Trick
from models.flights import Flight, FlightNew
from models.marks import JudgeMark, FinalMark
from models.competition_configs import CompetitionConfig
from models.results import RunResults, CompetitionResults, CompetitionPilotResults, RunResultSummary, CompetitionResultsExport
from models.cache import Cache

from core.database import db, PyObjectId
from core.config import settings
from core.utils import weight_average, average, ordinal

log = logging.getLogger(__name__)
collection = db.competitions

class CompetitionType(str, Enum):
    solo = 'solo'
    synchro = 'synchro'

class CompetitionState(str, Enum):
    init = 'init'
    open = 'open'
    closed = 'closed'

class CompetitionExport(BaseModel):
    id: str = Field(alias="_id")
    name: str
    code: str
    start_date: date
    end_date: date
    location: str
    published: bool
    type: CompetitionType
    pilots: List[Pilot]
    teams: List[TeamExport]
    judges: List[Judge]
    repeatable_tricks: List[Trick]
    state: CompetitionState
    config: CompetitionConfig
    runs: List[RunExport]
    image: Optional[AnyHttpUrl | str] = None
    logo: Optional[AnyHttpUrl | str] = None
    website: Optional[AnyHttpUrl] = None
    seasons: List[str]
    last_update: Optional[datetime] = None
    # TODO[pydantic]: The following keys were removed: `json_encoders`.
    # Check https://docs.pydantic.dev/dev-v2/migration/#changes-to-config for more information.
    model_config = ConfigDict(json_encoders={ObjectId: str})


class CompetitionPublicExport(BaseModel):
    id: str = Field(alias="_id")
    name: str
    code: str
    start_date: date
    end_date: date
    location: str
    published: bool
    type: CompetitionType
    state: CompetitionState
    number_of_pilots: int
    number_of_teams: int
    number_of_judges: int
    number_of_runs: int
    image: Optional[AnyHttpUrl | str] = None
    logo: Optional[AnyHttpUrl | str] = None
    website: Optional[AnyHttpUrl] = None
    seasons: List[str]
    last_update: Optional[datetime] = None
    # TODO[pydantic]: The following keys were removed: `json_encoders`.
    # Check https://docs.pydantic.dev/dev-v2/migration/#changes-to-config for more information.
    model_config = ConfigDict(json_encoders={ObjectId: str})

class CompetitionPublicExportWithResults(CompetitionPublicExport):
    results: CompetitionResultsExport
    pilots: List[Pilot]
    teams: List[TeamExport]
    judges: List[Judge]

class CompetitionNew(BaseModel):
    name: str = Field(..., min_len=1)
    code: Optional[str] = Field(None, pattern='^[a-z][a-z0-9-]*[a-z0-9]')
    start_date: date
    end_date: date
    location: str = Field(..., min_len=1)
    published: bool
    type: CompetitionType
    image: Optional[str] = None
    logo: Optional[str] = None
    website: Optional[AnyHttpUrl] = None
    seasons: List[str] = Field([])
    last_update: Optional[datetime] = None


    async def create(self):

        if self.code is None:
            self.code = re.sub(r'[^a-z0-9 ]', '', unicodedata.normalize("NFD", self.name).encode('ascii', 'ignore').decode('utf-8').lower()).strip().replace(' ', '-')

        competition = Competition(
            name = self.name,
            code = self.code,
            start_date = self.start_date,
            end_date = self.end_date,
            location = self.location,
            published = self.published,
            type = self.type,
            state = CompetitionState.init,
            config = CompetitionConfig(),
            repeatable_tricks = [str(trick.id) for trick in await Trick.getall(repeatable=True)],
            pilots = [],
            teams = [],
            judges = [],
            runs = [],
            deleted = None,
            image = self.image,
            logo = self.logo,
            website = self.website,
            seasons = self.seasons,
        )

        return await competition.create()

class Competition(CompetitionNew):
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    pilots: List[int] = Field([])
    teams: List[str] = Field([])
    judges: List[str] = Field([])
    repeatable_tricks: List[str] = Field([])
    state: CompetitionState
    config: CompetitionConfig
    runs: List[Run]
    deleted: Optional[datetime] = None
    image: Optional[str] = None
    logo: Optional[str] = None
    website: Optional[AnyHttpUrl] = None
    seasons: List[str] = Field([])
    # TODO[pydantic]: The following keys were removed: `json_encoders`.
    # Check https://docs.pydantic.dev/dev-v2/migration/#changes-to-config for more information.
    model_config = ConfigDict(populate_by_name=True, arbitrary_types_allowed=True, json_encoders={ObjectId: str}, json_schema_extra={
        "example": {
        }
    })

    async def check(self):
        if self.type == CompetitionType.solo:
            # ensure pilots exist and reverse order them by their respective rank
            # the order of the pilots represents the starting order of the 1st run
            pilots = []
            for id in self.pilots:
                pilot = await Pilot.get(id)
                if pilot is None:
                    raise HTTPException(400, f"Pilot '{id}' is unknown, only known pilots can take part of a competition")
                pilots.append(pilot)

            pilots.sort(key=lambda p: -p.rank)
            self.pilots = list(map(lambda p: p.civlid, pilots))

            if len(list(set(self.pilots))) != len(self.pilots):
                raise HTTPException(400, f"Can have duplicate pilots")

        if self.type == CompetitionType.synchro:
            for id in self.teams:
                team = await Team.get(id)
                if team is None:
                    raise HTTPException(400, f"Team '{id}' is unknown, only known teams can take part of a competition")

            if len(list(set(self.teams))) != len(self.teams):
                raise HTTPException(400, f"Can have duplicate teams")

        for id in self.judges:
            judge = await Judge.get(id)
            if judge is None:
                raise HTTPException(400, f"Judge '{id}' is unknown, only known judges can take part of a competition")

        if self.state != CompetitionState.init:
            if self.type == CompetitionType.solo and len(self.pilots) < 2:
                raise HTTPException(400, "At least 2 pilots are needed to open a competition")
            if self.type == CompetitionType.synchro and len(self.teams) < 2:
                raise HTTPException(400, "At least 2 teams are needed to open a competition")
            if len(self.judges) < 2:
                raise HTTPException(400, "At least 2 judges are needed to open a competition")

        if self.start_date > self.end_date:
            raise HTTPException(400, "End date must be higher than start date")

    async def create(self):
        try:
            self.deleted = None
            self.runs = []
            await self.check()
            competition = jsonable_encoder(self)
            res = await collection.insert_one(competition)
            self.id = res.inserted_id
            return self
        except pymongo.errors.DuplicateKeyError:
            raise HTTPException(400, f"Competition '{self.name}' already exists")

    async def save(self):
        await self.check()
        competition = jsonable_encoder(self)

        old = await collection.find_one({"_id": str(self.id)})
        if old and competition == old:
            return

        res = await collection.update_one({"_id": str(self.id)}, {"$set": competition})
        if res.modified_count != 1:
            raise HTTPException(400, f"Error while saving Competition {self.id}, 1 item should have been saved, got {res.modified_count}")

    def image_url(self):
        if self.image is not None:
            return f"/files/{self.image}"
        return None

    def logo_url(self):
        if self.logo is not None:
            return f"/files/{self.logo}"
        return None


    async def export(self, cache:Cache = None) -> CompetitionExport:

        pilots = []
        if self.type == CompetitionType.solo:
            for pilot in self.pilots:
                pilots.append(await Pilot.get(pilot, cache=cache))

        teams = []
        if self.type == CompetitionType.synchro:
            for team in self.teams:
                team = await Team.get(team, cache=cache)
                teams.append(await team.export(cache=cache))

        judges = []
        for judge in self.judges:
            judges.append(await Judge.get(judge, cache=cache))

        repeatable_tricks = []
        for trick in self.repeatable_tricks:
            repeatable_tricks.append(await Trick.get(trick, deleted=(self.state == CompetitionState.closed), cache=cache))


        runs = []
        for run in self.runs:
            runs.append(await run.export(cache=cache))

        return CompetitionExport(
            _id = str(self.id),
            name = self.name,
            code = self.code,
            start_date = self.start_date,
            end_date = self.end_date,
            location = self.location,
            published = self.published,
            type = self.type,
            pilots = pilots,
            teams = teams,
            judges = judges,
            repeatable_tricks = repeatable_tricks,
            state = self.state,
            config = self.config,
            runs = runs,
            image = self.image_url(),
            logo = self.logo_url(),
            website = self.website,
            seasons = self.seasons,
            last_update = self.last_update,
        )

    async def export_public(self, cache:Cache = None) -> CompetitionPublicExport:
        if not self.published:
            return None

        return CompetitionPublicExport(
            _id = str(self.id),
            name = self.name,
            code = self.code,
            start_date = self.start_date,
            end_date = self.end_date,
            location = self.location,
            published = self.published,
            type = self.type,
            number_of_pilots = len(self.pilots),
            number_of_teams = len(self.teams),
            number_of_judges = len(self.judges),
            number_of_runs = len(self.runs),
            state = self.state,
            image = self.image_url(),
            logo = self.logo_url(),
            website = self.website,
            seasons = self.seasons,
            last_update = self.last_update,
        )

    async def export_public_with_results(self, cache:Cache = None) -> CompetitionPublicExportWithResults:
        if not self.published:
            raise HTTPException(404, f"Competition {self.code} not published")

        results = await self.results()
        for result in results.runs_results:
            for result_type in result.results:
                for r in result.results[result_type]:
                    r.marks = []

        comp = await self.export(cache=cache)
        return CompetitionPublicExportWithResults(
            _id = str(comp.id),
            name = comp.name,
            code = comp.code,
            start_date = comp.start_date,
            end_date = comp.end_date,
            location = comp.location,
            published = comp.published,
            type = comp.type,
            pilots = comp.pilots,
            teams = comp.teams,
            judges = comp.judges,
            number_of_pilots = len(self.pilots),
            number_of_teams = len(self.teams),
            number_of_judges = len(self.judges),
            number_of_runs = len(self.runs),
            state = comp.state,
            results = await results.export(cache=cache),
            image = comp.image,
            logo = comp.logo,
            website = comp.website,
            seasons = comp.seasons,
            last_update = self.last_update,
        )

#    async def sort_pilots(self):
#        if len(self.pilots) == 0:
#            return
#
#        pilots = []
#        # Pilot.getall return a sorted list of pilots by rank and name
#        for pilot in await Pilot.getall(self.pilots):
#            pilots.append(pilot.name)
#        self.pilots = pilots

    async def update(self, updated_comp: CompetitionNew):
        if self.state == CompetitionState.closed:
            raise HTTPException(400, "Can't change on a closed competition")

        self.start_date = updated_comp.start_date
        self.end_date = updated_comp.end_date
        self.location = updated_comp.location
        self.published = updated_comp.published
        self.image = updated_comp.image
        if self.image is not None:
            self.image = self.image.split('/')[-1]
        self.logo = updated_comp.logo
        if self.logo is not None:
            self.logo = self.logo.split('/')[-1]
        self.website = updated_comp.website
        self.seasons = updated_comp.seasons

        if self.type != updated_comp.type and self.state != CompetitionState.init:
            raise HTTPException(400, "Can't change the type of an already open or closed competition")
        self.type = updated_comp.type

        if self.name != updated_comp.name and self.state != CompetitionState.init:
            raise HTTPException(400, "Can't change the name of an already open or closed competition")
        self.name = updated_comp.name

        if self.code != updated_comp.code and self.state != CompetitionState.init:
            raise HTTPException(400, "Can't change the code of an already open or closed competition")
        self.code = updated_comp.code

        await self.save()

    async def update_pilots(self, pilots: List[int]):
        if self.state == CompetitionState.closed:
            raise HTTPException(400, "Can't update pilots of a closed competition")

        if self.type != CompetitionType.solo:
            raise HTTPException(400, "Pilot's list can only be changed on a solo competition")
        self.pilots = pilots
        await self.save()

    async def update_teams(self, teams: List[str]):
        if self.state == CompetitionState.closed:
            raise HTTPException(400, "Can't update teams of a closed competition")

        if self.type != CompetitionType.synchro:
            raise HTTPException(400, "Team's list can only be changed on a synchro competition")
        self.teams = teams
        await self.save()

    async def update_judges(self, judges: List[str]):
        if self.state == CompetitionState.closed:
            raise HTTPException(400, "Can't update judges of a closed competition")

        self.judges = judges
        await self.save()

    async def update_repeatable_tricks(self, repeatable_tricks: List[str]):
        if self.state == CompetitionState.closed:
            raise HTTPException(400, "Can't update repeatable tricks of a closed competition")

        self.repeatable_tricks = repeatable_tricks
        await self.save()

    async def update_config(self, config: CompetitionConfig):
        if self.state == CompetitionState.closed:
            raise HTTPException(400, "Can't change configuration of a closed competition")
        self.config = config
        await self.save()

    async def open(self):
        if self.state != CompetitionState.init:
            raise HTTPException(400, "Can't open a comp which is not in state 'init'")

        self.state = CompetitionState.open
        await self.save()

    async def close(self):
        if self.state != CompetitionState.open:
            raise HTTPException(400, "Can't close a comp which is not in state 'open'")

        self.state = CompetitionState.closed
        await self.save()

    async def reopen(self):
        if self.state != CompetitionState.closed:
            raise HTTPException(400, "Can't reopen a comp which is not in state 'closed'")

        self.state = CompetitionState.open
        await self.save()


    def get_pilot_or_team_rank_in_current_overall(self, pilot_or_team, results) -> int:
        for rank, result in enumerate(results):
            if self.type == CompetitionType.solo:
                if result.pilot == pilot_or_team:
                    return rank+1
            else:
                if result.team == pilot_or_team:
                    return rank+1
        return -1


    async def new_run(self, pilots_to_qualify: int = 0, repetitions_reset_policy: RunRepetitionsResetPolicy = RunRepetitionsResetPolicy.none, saveToDB: bool = True):
        if self.state != CompetitionState.open:
            raise HTTPException(400, "Competition must be 'open' to create a new run")


        pilots = []
        teams = []
        if self.type == CompetitionType.solo:
            pilots = self.pilots
        else:
            teams = self.teams

        if len(self.runs) > 0: # take the list of pilots from the previous run

            # get overall results from the previous runs to order pilots/teams
            results = await self.results()

            has_awt = [x for x in results.results.keys() if x.startswith('awt-')]
            has_awq = [x for x in results.results.keys() if x.startswith('awq-')]

            # if this is a mix event with AWT and AWQ, separate starting order
            # with AWQ first, then AWT, never mix both categories
            if self.type == CompetitionType.solo and len(has_awt) == 1 and len(has_awq) == 1:
                awt_results = results.results[has_awt[0]]
                awq_results = results.results[has_awq[0]]
                results = awt_results + awq_results
            else:
                results = results.results["overall"]


            if self.type == CompetitionType.solo:
                pilots = self.runs[-1].pilots
                pilots.sort(key=lambda p:-self.get_pilot_or_team_rank_in_current_overall(p, results))
            else:
                teams = self.runs[-1].teams
                teams.sort(key=lambda t:-self.get_pilot_or_team_rank_in_current_overall(t, results))

        else: #  first run to be added, use the list of pilots of the competition
            if self.type == CompetitionType.solo:
                pilots = self.pilots
            else:
                # TOOD: order by team name, fine tuning the order will be made manually
                teams = self.teams

        if pilots_to_qualify > 0: # only keep the N first (best) pilots/teams of the list
            pilots = pilots[0:pilots_to_qualify]
            teams = teams[0:pilots_to_qualify]

        run = Run(
            state=RunState.init,
            type=self.type,
            pilots=pilots,
            teams=teams,
            judges=self.judges,
            config=self.config,
            repeatable_tricks=self.repeatable_tricks,
            flights=[],
            repetitions_reset_policy=repetitions_reset_policy,
        )
        self.runs.append(run)
        if saveToDB:
            await self.save()
        return run

    async def run_get(self, i: int) -> Run:
        try:
            return self.runs[i]
        except IndexError:
            raise HTTPException(404, "Run #{i} not found in comp {self.id}")

    async def run_update_pilots(self, i: int, pilots: List[int]):
        run = await self.run_get(i)
        if self.type != CompetitionType.solo:
            raise HTTPException(400, "Pilot's list can only be changed on a solo run")
        run.pilots = pilots
        self.runs[i] = run
        await self.save()

    async def run_update_teams(self, i: int, teams: List[str]):
        run = await self.run_get(i)
        if self.type != CompetitionType.synchro:
            raise HTTPException(400, "Team's list can only be changed on a synchro run")
        run.teams = teams
        self.runs[i] = run
        await self.save()

    async def run_update_judges(self, i: int, judges: List[str]):
        run = await self.run_get(i)
        run.judges = judges
        self.runs[i] = run
        await self.save()

    async def run_update_repeatable_tricks(self, i: int, repeatable_tricks: List[str]):
        run = await self.run_get(i)
        run.repeatable_tricks = repeatable_tricks
        self.runs[i] = run
        await self.save()

    async def run_update_config(self, i: int, config: CompetitionConfig):
        run = await self.run_get(i)
        run.config = config
        self.runs[i] = run
        await self.save()

    async def run_open(self, i: int):
        run = await self.run_get(i)

        if run.state != RunState.init:
            raise HTTPException(400, "Can't open a run which is not in state 'init'")

        run.state = RunState.open
        self.runs[i] = run
        await self.save()

    async def run_close(self, i: int):
        run = await self.run_get(i)

        if run.state != RunState.open:
            raise HTTPException(400, "Can't close a run which is not in state 'open'")

        run.state = RunState.closed
        self.runs[i] = run
        await self.save()

    async def run_reopen(self, i: int):
        run = await self.run_get(i)

        if run.state != RunState.closed:
            raise HTTPException(400, "Can't reopen a run which is not in state 'close'")

        run.state = RunState.open
        self.runs[i] = run
        await self.save()

    async def run_results(self, run_i: int, published_only: bool = True) -> RunResults:
        run = await self.run_get(run_i)

        all_published=True
        overall = {}
        for flight in run.flights:
            if not flight.published:
                all_published=False
                if published_only:
                  continue
            if self.type == CompetitionType.solo:
                overall[flight.pilot] = flight
            else:
                overall[flight.team] = flight

        overall_results = list(overall.values())
        overall_results.sort(key=lambda e: e.final_marks.score)

        results = {}
        results["overall"] = overall_results[::-1]

        pilots = []
        if self.type == CompetitionType.solo:
            pilots = await Pilot.getall(list(overall.keys()))

        self.create_sub_results(results, pilots)

        return RunResults(
            results = results,
            type = self.type,
            final = (run.state == RunState.closed) and all_published,
            last_update = run.last_update,
        )

    async def flight_get(self, run_i: int, pilot_or_team) -> Flight:
        run = await self.run_get(run_i)

        if self.type == CompetitionType.solo:
            if int(pilot_or_team) not in run.pilots:
                raise HTTPException(400, f"Pilot #{pilot_or_team} does not participate in the run number #{run_i} of the comp ({self.name})")
            if int(pilot_or_team) not in self.pilots:
                raise HTTPException(400, f"Pilot #{pilot_or_team} does not participate in this comp ({self.name})")

        if self.type == CompetitionType.synchro:
            if pilot_or_team not in run.teams:
                raise HTTPException(400, f"Team #{pilot_or_team} does not participate in the run number #{run_i} of the comp ({self.name})")
            if pilot_or_team not in self.teams:
                raise HTTPException(400, f"Team #{pilot_or_team} does not participate in this comp ({self.name})")

        for i, f in enumerate(self.runs[run_i].flights):
            if (self.type == CompetitionType.solo and f.pilot == int(pilot_or_team)) or (self.type == CompetitionType.synchro and f.team == pilot_or_team):
                flight = self.runs[run_i].flights[i]
                return flight

        raise HTTPException(404, "Flight not found")

    async def flight_delete(self, run_i: int, pilot_or_team) -> Flight:
        flight = await self.flight_get(run_i, pilot_or_team)
        self.runs[run_i].flights.remove(flight)
        await self.save()

    async def flight_convert(self, id, flight: FlightNew) -> Flight:
        tricks = []
        errors = []
        for trick_name in flight.tricks:
            trick = await Trick.get_unique_trick(trick_name)
            if trick is None:
                errors.append(trick_name)
            else:
                tricks.append(trick)

        if len(errors) > 0:
            errors = ' '.join(errors)
            raise HTTPException(400, f"Unknown trick(s): {errors}")

        teamid=""
        pilotid=0
        if self.type == CompetitionType.solo:
            pilotid=id
        if self.type == CompetitionType.synchro:
            teamid=id

        return Flight(
                pilot = pilotid,
                team = teamid,
                tricks = tricks,
                marks = flight.marks,
                did_not_start = flight.did_not_start,
                warnings = flight.warnings,
        )

    async def flight_save(self, run_i: int, id, flight: FlightNew, save: bool=False, published: bool=False, saveToDB : bool = True, mark_type:str = None, cache:Cache = None) -> FinalMark:
        run = await self.run_get(run_i)

        is_awt = False

        if self.type == CompetitionType.solo:
            if int(id) not in run.pilots:
                raise HTTPException(400, f"Pilot #{id} does not participate in the run number #{run_i} of the comp ({self.name})")
            if int(id) not in self.pilots:
                raise HTTPException(400, f"Pilot #{id} does not participate in this comp ({self.name})")

            pilot = await Pilot.get(int(id))
            is_awt = pilot.is_awt(self.start_date.year)

        if self.type == CompetitionType.synchro:
            if id not in run.teams:
                raise HTTPException(400, f"Team #{id} does not participate in the run number #{run_i} of the comp ({self.name})")
            if id not in self.teams:
                raise HTTPException(400, f"Team #{id} does not participate in this comp ({self.name})")

        new_flight = await self.flight_convert(id, flight)
        mark = await self.calculate_score(flight=new_flight, run_i=run_i, is_awt_pilot=is_awt, mark_type=mark_type, cache=cache)
        if not save:
            return mark

        new_flight.final_marks = mark
        new_flight.published = published
        new_flight.last_update = datetime.now()
        self.last_update = new_flight.last_update

        for i, f in enumerate(self.runs[run_i].flights):
            if (self.type == CompetitionType.solo and f.pilot == new_flight.pilot) or (self.type == CompetitionType.synchro and f.team == new_flight.team):
                self.runs[run_i].flights[i] = new_flight
                self.runs[run_i].last_update = new_flight.last_update
                if saveToDB:
                    await self.save()
                return mark

        self.runs[run_i].flights.append(new_flight)
        self.runs[run_i].last_update = new_flight.last_update
        if saveToDB:
            await self.save()
        return mark

    async def results(self, limit: int = -1) -> CompetitionResults:
        final = (self.state == CompetitionState.closed)
        overall_results = []
        runs_results = []

        overall = {}

        for i, run in enumerate(self.runs):
            if limit >= 0 and i > limit:
                break
            run_result = await self.run_results(i, published_only=False)
            runs_results.append(run_result)
            if not run_result.final:
                final = False

            for j, result in enumerate(run_result.results["overall"]):

                run_result_summary = RunResultSummary(
                    rank = len(run_result.results["overall"])-j,
                    score = result.final_marks.score
                )

                if self.type == CompetitionType.solo:
                    if result.pilot not in overall:
                        overall[result.pilot] = CompetitionPilotResults(
                            pilot=result.pilot,
                            team='',
                            score=0,
                            result_per_run=[]
                        )

                    overall[result.pilot].score = round(overall[result.pilot].score + result.final_marks.score, 3)
                    overall[result.pilot].result_per_run.append(run_result_summary)

                if self.type == CompetitionType.synchro:
                    if result.team not in overall:
                        overall[result.team] = CompetitionPilotResults(
                            pilot=0,
                            team=result.team,
                            score=0,
                            result_per_run=[]
                        )

                    overall[result.team].score = round(overall[result.team].score + result.final_marks.score, 3)
                    overall[result.team].result_per_run.append(run_result_summary)

        overall_results = list(overall.values())
        overall_results.sort(key=lambda e: e.score)

        results = {}
        results["overall"] = overall_results[::-1]

        pilots = []
        if self.type == CompetitionType.solo:
            pilots = await Pilot.getall(list(map(lambda r: r.pilot, overall_results)))

        self.create_sub_results(results, pilots)
        for run_results in runs_results:
            self.create_sub_results(run_results.results, pilots)

        #
        #  update result_per_run for each sub rankings
        #
        for result_type in list(results):

            if result_type == 'overall':
                continue

            for result in results[result_type]:

                result_per_run = []

                for (run_index, run) in enumerate(runs_results):

                    if run.results[result_type] is None:
                        continue

                    for (rank_index, run_results) in enumerate(run.results[result_type]):

                        if (self.type == CompetitionType.solo and result.pilot == run_results.pilot) or (self.type == CompetitionType.synchro and result.team == run_results.team):
                            result_per_run.append(RunResultSummary(rank=rank_index+1, score=run_results.final_marks.score))

                result.result_per_run = result_per_run

        return CompetitionResults(
            final = final,
            type = self.type,
            results = results,
            runs_results = runs_results,
        )

    def create_sub_results(self, results, pilots):
        # seasons
        for season in self.seasons:

            if self.type == CompetitionType.synchro:
                results[season] = results['overall']
                continue

            if re.search('^awt-\d{4}$', season):
                awt_pilots = list(filter(lambda p: p.is_awt(self.start_date.year), pilots))
                awt_results = list(filter(lambda r: next((p for p in awt_pilots if p.civlid == r.pilot), None) is not None, results['overall']))
                awt_results.sort(key=lambda e: e.score if hasattr(e, 'score') else e.final_marks.score)
                results[season] = list(awt_results[::-1])
                continue

            if re.search('^awq-\d{4}$', season):
                awq_pilots = list(filter(lambda p: not p.is_awt(self.start_date.year), pilots))
                awq_results = list(filter(lambda r: next((p for p in awq_pilots if p.civlid == r.pilot), None) is not None, results['overall']))
                awq_results.sort(key=lambda e: e.score if hasattr(e, 'score') else e.final_marks.score)
                results[season] = list(awq_results[::-1])
                continue

            if re.search('^\w\w\w-\d{4}$', season):
                country = season[0:3]
                country_pilots = list(filter(lambda p: p.country == country, pilots))
                country_results = list(filter(lambda r: next((p for p in country_pilots if p.civlid == r.pilot), None) is not None, results['overall']))
                country_results.sort(key=lambda e: e.score if hasattr(e, 'score') else e.final_marks.score)
                results[season] = list(country_results[::-1])
                continue


        # women results
        if self.type == CompetitionType.solo:
            women_pilots = list(filter(lambda p: p.gender == 'woman', pilots))
            women_results = list(filter(lambda r: next((p for p in women_pilots if p.civlid == r.pilot), None) is not None, results['overall']))
            if len(women_pilots) >= 3:
                women_results.sort(key=lambda e: e.score if hasattr(e, 'score') else e.final_marks.score)
                results["women"] = list(women_results[::-1])


    #
    #
    #   Static methods
    #
    #
    @staticmethod
    def createIndexes():
        collection.create_index([('name', pymongo.ASCENDING), ('deleted', pymongo.ASCENDING)], unique=True)
        collection.create_index([('code', pymongo.ASCENDING), ('deleted', pymongo.ASCENDING)], unique=True)
        log.debug('index created on "name,deleted" and on "code,deleted"')

    @staticmethod
    async def get(id: str, deleted: bool = False, cache:Cache = None):
        search = {"$or": [{"_id": id}, {"code": id}]}
        if not deleted:
            search['deleted'] = None
            if cache is not None:
                competition = cache.get('competitions', id)
                if competition is not None:
                    return competition
        competition = await collection.find_one(search)
        if competition is None:
            raise HTTPException(404, f"Competition {id} not found")
        competition = Competition.model_validate(competition)
        if not deleted and cache is not None:
            cache.add('competitions', competition)
        return competition

    @staticmethod
    async def getall(season: str = None, cache:Cache = None):
        if cache is not None:
            competitions = cache.get_all('competitions')
            if competitions is not None:
                if season is None:
                    return competitions
                else:
                    return [c for c in competitions if season in c.seasons]

        competitions = []
        for competition in await collection.find({"deleted": None}, sort=[("name", pymongo.ASCENDING)]).to_list(1000):
            competition = Competition.model_validate(competition)
            if season is None or season in competition.seasons:
                competitions.append(competition)
                if cache is not None:
                    cache.add('competitions', competition)

        if cache is not None and season is None:
            cache.set_all('competitions', competitions)
        return competitions

    @staticmethod
    async def delete(id: str, restore: bool = False):
        competition = await Competition.get(id, True)
        if competition is None:
            return None

        if restore ^ (competition.deleted is not None):
            if restore:
                raise HTTPException(400, f"Can't restore Competition {id} as it's not deleted")
            else:
                raise HTTPException(400, f"Can't delete Competition {id} as it's already deleted")

        if restore:
            competition.deleted = None
        else:
            competition.deleted = datetime.now()
        return await competition.save()

    def is_awt_season(self):
        return bool([s for s in self.seasons if re.match('^awt-', s)])

    def is_awq_season(self):
        return bool([s for s in self.seasons if re.match('^awq-', s)])


    async def calculate_score(self, flight: Flight, run_i: int = -1, is_awt_pilot=False, mark_type:str = None, cache:Cache = None) -> FinalMark:
        mark = FinalMark(
            judges_mark = JudgeMark(
                judge = "",
                technical = 0,
                choreography = 0,
                landing = 0,
                synchro = 0,
            ),
            technicity=0,
            bonus_percentage=0,
            technical=0,
            choreography=0,
            landing=0,
            synchro=0,
            bonus=0,
            score=0,
            warnings=flight.warnings,
            malus=0,
            mark_type=mark_type,
        )

        # for lukas :-)
        if flight.did_not_start:
            mark.notes.append(f"Got 0 because of DNS")
            return mark

        if run_i < 0:
            run = None
        else:
            try:
                run = self.runs[run_i]
            except IndexError:
                run = None

        if run is None:
            config = self.config
        else:
            config = run.config

        # get judge details
        judges = {}
        for m in flight.marks:
            if m.judge not in judges:
                judge = await Judge.get(m.judge, cache=cache)
                if judge is None:
                    raise HTTPException(400, f"judge '{m.judge}' not found")
                judges[m.judge] = judge

        # check that we have all technical marks for every judges
        number_of_marks_without_technical_per_trick = len(flight.marks)
        for m in flight.marks:
            judge = judges[m.judge].name
            if m.technical_per_trick is None:
                number_of_marks_without_technical_per_tricki -= 1
                continue

            n_technical_per_trick = len(m.technical_per_trick)
            n_tricks = len(flight.tricks)
            if n_technical_per_trick != n_tricks:
                raise HTTPException(400, f"judge '{judge}' did not mark enough tricks ({n_technical_per_trick} instead of {n_tricks})")

        if number_of_marks_without_technical_per_trick > 0 and number_of_marks_without_technical_per_trick < len(flight.marks):
            raise HTTPException(400, f"somes judges sent technical mark per trick while other(s) did not")

        # only for marks with technical_marks
        if number_of_marks_without_technical_per_trick > 0:
            # when a mark is missing
            # try to make average from other marks if possible

            # iterate over tricks
            for i, trick in enumerate(flight.tricks):

                # retrieve marks for this trick
                marks = [m.technical_per_trick[i] for m in flight.marks]

                # if all marks are set, no need to guess anything
                if len([m for m in marks if m is None]) == 0:
                    continue

                # retrieve marks that are set
                set_marks = [m for m in marks if m is not None]

                # if no mark have been set for this trick
                if len(set_marks) == 0:
                    raise HTTPException(400, f"not enough marks for {trick.name}")

                # guess the mark from the average of the others
                # and set it to replace unset marks
                guessed_mark = average(set_marks)
                for m in flight.marks:
                    judge = judges[m.judge].name
                    if m.technical_per_trick[i] is None:
                        m.technical_per_trick[i] = guessed_mark
                        log.error(f"{judge}/{trick.name} set mark to {guessed_mark}")


        if mark_type in ["awq", "awt"]:
            # associate technical mark to each trick
            for i, trick in enumerate(flight.tricks):
                trick.technical_mark = None
                trick.technical_marks = {}
                for m in flight.marks:
                    if m.technical_per_trick is None:
                        continue
                    if i >= len(m.technical_per_trick):
                        continue
                    trick.technical_marks[m.judge] = m.technical_per_trick[i]

                if len(trick.technical_marks) == 0:
                    raise HTTPException(400, f"no technical mark for trick {trick.acronym}")

                # calculate technical average for this trick
                if mark_type == "awt" and len(trick.technical_marks) > 0:
                    technicals = []
                    for j, m in trick.technical_marks.items():
                        judge = judges[j]
                        weight = dict(config.judge_weights)[judge.level.value]
                        technicals.append((m, weight))
                    trick.technical_mark = weight_average(technicals)


        #
        # check if a trick is not perform in a allowed position (not the first or not as a last x maneuvers)
        #
        for i, trick in enumerate(flight.tricks):
            t = await Trick.get(trick.base_trick)

            # the trick MUST be performed in the X first maneuvers of the run
            if t.first_maneuver > 0 and i >= t.first_maneuver:
                if t.first_maneuver == 1:
                    mark.warnings.append(f"{trick.name} must be the first maneuver")
                else:
                    mark.warnings.append(f"{trick.name} must be one of the first {t.first_maneuver} maneuvers")

            # the trick can't be performed in the X first maneuvers of the run
            if t.no_first_maneuver > 0 and i < t.no_first_maneuver:
                if t.no_first_maneuver == 1:
                    mark.warnings.append(f"{trick.name} can't be the first maneuver")
                else:
                    mark.warnings.append(f"{trick.name} can't be one of the first {t.no_first_maneuver} maneuvers")

            # the trick MUST be performed in the X last maneuvers of the run
            if t.last_maneuver > 0 and i >= len(flight.tricks) - t.last_maneuver:
                if t.last_maneuver == 1:
                    mark.warnings.append(f"{trick.name} must be the last maneuver")
                else:
                    mark.warnings.append(f"{trick.name} must be one of the last {t.last_maneuver} maneuvers")

            # the trick can't be performed in the X last maneuvers of the run
            if t.no_last_maneuver > 0 and i >= len(flight.tricks) - t.no_last_maneuver:
                if t.no_last_maneuver == 1:
                    mark.warnings.append(f"{trick.name} can't be the last maneuver")
                else:
                    mark.warnings.append(f"{trick.name} can't be one of the last {t.no_last_maneuver} maneuvers")
        #
        # endof check if a trick is not perform in a allowed position (not the first or not as a last x maneuvers)
        #


        #
        # count previous warnings and check if not previous DSQ
        #
        previous_warnings = 0
        if run_i>0 and run is not None:

            # loop over all previous runs
            for i in range(len(self.runs)):
                if i >= run_i:
                    break
                r = self.runs[i]
                for f in r.flights:
                    if (self.type == CompetitionType.solo and flight.pilot == f.pilot) or (self.type == CompetitionType.synchro and flight.team == f.team):
                        previous_warnings += len(f.warnings)
                        break

            if previous_warnings >= config.warnings_to_dsq:
                mark.notes.append(f"Pilot has been DSQ because he/she already had {config.warnings_to_dsq} warnings")
                return mark

        if len(mark.warnings) + previous_warnings >= config.warnings_to_dsq:
            mark.notes.append(f"Pilot has been DSQ he/she's got {config.warnings_to_dsq} warnings")
            return mark
        #
        # end of checking warning to DSQ
        #


        # calculate the average of judges marks
        # using the weight of each judge level
        technicals = []
        choreographies = []
        landings = []
        synchros = []
        for m in flight.marks:
            judge = await Judge.get(m.judge)
            if judge is None:
                raise HTTPException(400, f"judge '{m.judge}' not found")
            weight = dict(config.judge_weights)[judge.level.value]
            if m.technical is not None:
                technicals.append((m.technical, weight))
            if m.choreography is not None:
                choreographies.append((m.choreography, weight))
            if m.landing is not None:
                landings.append((m.landing, weight))
            if self.type == CompetitionType.synchro:
                if m.synchro is not None:
                    synchros.append((m.synchro, weight))

        if len(technicals) == 0 and mark_type not in ["awq", "awt"]:
            raise HTTPException(400, f"not enough technical marks")

        if len(choreographies) == 0:
            raise HTTPException(400, f"not enough choreography marks")

        if len(landings) == 0:
            raise HTTPException(400, f"not enough landing marks")

        if self.type == CompetitionType.synchro and len(synchros) == 0:
            raise HTTPException(400, f"not enough synchro marks")

        mark.judges_mark.technical = weight_average(technicals)
        mark.judges_mark.choreography = weight_average(choreographies)
        mark.judges_mark.landing = weight_average(landings)
        if self.type == CompetitionType.synchro:
            mark.judges_mark.synchro = weight_average(synchros)
        #
        # endof calculating the weight average of the judges marks
        #


        #
        # tricks direction odd-even check
        #
        # judging marks must always put 1 for left/right odd-even choreo point
        # and for solo only, lower the choreo by 1 point of left/right is not even
        #
        if self.type != CompetitionType.synchro and len(flight.tricks) > 1: # this does not make sense with only 1 trick
            directions = []
            available_directions = list(map(lambda  x:x['name'], settings.tricks.available_directions))

            # parse each tricks
            for trick in flight.tricks:

                # search if there is a direction
                for direction in trick.uniqueness:
                    if direction in available_directions:
                        directions.append(direction)
                        break

            # if there are more than 1 direction trick
            if len(directions) > 1:

                # count the number of each directions
                directions = Counter(directions)
                # only keep the number of each directions
                directions = directions.values()
                # len(directions) == 1 --> 2 or more tricsk to the same direction
                # max(directions) - min(directions) > 1 --> is number of tricks with direction even ?
                if len(directions) == 1 or (max(directions) - min(directions)) > 1:
                    # then lower the choreography mark by 1
                    if mark.judges_mark.choreography >= 1:
                        mark.notes.append(f"choreography mark has been lower by 1 point because direction is not even")
                        mark.judges_mark.choreography -= 1
        #
        # endof tricks direction odd-even check
        #


        #
        # search for repetitions
        # §6.5.1 from 7B
        # each trick can be performed left/right and reversed without malus
        # during the same competition
        # search in the previous runs
        # and in the current un from the previous tricks flown
        #

        repeatable_tricks=[]
        for trick in self.repeatable_tricks:
            trick = await Trick.get(trick)
            if trick is not None:
                repeatable_tricks.append(trick.name)

        first_run_to_check_repetitions = 0
        if self.type == CompetitionType.solo:
            for i in range(len(self.runs)):
                if self.runs[i].repetitions_reset_policy == RunRepetitionsResetPolicy.none:
                    continue
                if self.runs[i].repetitions_reset_policy == RunRepetitionsResetPolicy.all:
                    first_run_to_check_repetitions = i
                    continue
                if self.runs[i].repetitions_reset_policy == RunRepetitionsResetPolicy.awt and is_awt_pilot and self.is_awt_season():
                    first_run_to_check_repetitions = i
                    continue
                if self.runs[i].repetitions_reset_policy == RunRepetitionsResetPolicy.awq and not is_awt_pilot and self.is_awq_season():
                    first_run_to_check_repetitions = i
                    continue

        if len(self.runs) > 0 and run_i > 0:
            trick_i = 0
            for trick in flight.tricks: # for each trick detect repetition before
                trick_i += 1
                if trick.base_trick in repeatable_tricks:
                    continue
                # loop over all previous runs
                for i in range(len(self.runs)):
                    if i < first_run_to_check_repetitions:
                        continue
                    if i >= run_i:
                        break
                    r = self.runs[i]
                    broke = False
                    for f in r.flights:
                        if (self.type == CompetitionType.solo and flight.pilot != f.pilot) or (self.type == CompetitionType.synchro and flight.team != f.team):
                            continue
                        for t in f.tricks:
                            if t.base_trick == trick.base_trick and t.uniqueness == trick.uniqueness:
                                mark.malus += config.malus_repetition
                                mark.notes.append(f"trick number #{trick_i} ({trick.name}) has already been performed in a previous run. Adding a {config.malus_repetition}% malus.")
                                broke = True
                                break
                    if broke:
                        break

        trick_i = 0
        for trick in flight.tricks: # for each trick detect repetition before
            trick_i += 1
            if trick.base_trick in repeatable_tricks:
                continue
            t_i = 0
            for t in flight.tricks:
                t_i += 1
                if t_i >= trick_i:
                    break
                if t.base_trick == trick.base_trick and t.uniqueness == trick.uniqueness:
                    mark.malus += config.malus_repetition
                    mark.notes.append(f"trick number #{trick_i} ({trick.name}) has already been performed in this run. Adding a {config.malus_repetition}% malus.")
                    break
        #
        # endof search for repetitions
        #


        #
        # ignore tricks
        #
        tricks = [] # the list of tricks that will be used to calculate the scores
        technical_marks_per_judges = {}
        n_bonuses = {}
        limits_per_type = {}
        for i, trick in enumerate(flight.tricks):
            ignoring = False

            #
            # ignore tricks by competition types
            #
            if self.type == CompetitionType.solo:
                if not trick.solo:
                    mark.notes.append(f"Ignoring trick #{i} ({trick.name}) because it's not allowed in a solo competition")
                    ignoring = True

                elif is_awt_pilot and self.is_awt_season() and not trick.solo_awt:
                    mark.notes.append(f"Ignoring trick #{i} ({trick.name}) because it's not allowed in AWT solo competition")
                    ignoring = True

            elif self.type == CompetitionType.synchro and not trick.synchro:
                mark.notes.append(f"Ignoring trick #{i} ({trick.name}) because it's not allowed in a synchro competition")
                ignoring = True

            if not ignoring:
                #
                # ignore tricks with bonus higher than the maximum bonus tricks allowed
                #
                for bonus_type in trick.bonus_types:
                    if bonus_type not in n_bonuses:
                        n_bonuses[bonus_type] = 0

                    n_bonuses[bonus_type] += 1

                    max_per_run=10
                    try:
                        max_per_run = dict(config.max_bonus_per_run)[bonus_type]
                    except:
                        pass

                    if n_bonuses[bonus_type] > max_per_run:
                        mark.notes.append(f"trick number #{i} ({trick.name}) has been ignored because more than {max_per_run} {bonus_type} tricks have been flown")
                        ignoring = True

                #
                # endof ignore tricks max_bonus_per_run
                #

                #
                # 6.5.1.2 (2024) ignore tricks that can't be performed during the same run
                #
                for _type in trick.types:
                    limit = settings.competitions.max_tricks_per_type.get(_type, None)
                    if limit is not None:
                        limits_per_type.setdefault(_type, 0)
                        limits_per_type[_type] += 1
                        if limits_per_type[_type] > limit:
                            mark.notes.append(f"trick number #{i} ({trick.name}) has been ignored because more than {limit} '{_type}' tricks have been flown")
                            ignoring = True
                #
                # endof 6.5.1.2 (2024) ignore tricks that can't be performed during the same run
                #

            if not ignoring:
                tricks.append(trick)
        #
        # endof ignore tricks
        #



        #
        # handle the technical mark per trick (as per §6.6.1)
        # technical marks = average of marks of each trick
        #
        if mark_type == "awt":
            technical_marks = []
            for trick in tricks:
                if trick.technical_mark is None:
                    raise HTTPException(400, f"not enough technicals marks for trick {trick.acronym}")
                technical_marks.append(trick.technical_mark)

            if len(technical_marks) != len(tricks):
                raise HTTPException(400, f"not enough technicals marks")
            mark.judges_mark.technical = average(technical_marks)

        elif mark_type == "awq":
            technical_marks_per_judge = {}
            for trick in tricks:
                if len(trick.technical_marks) == 0:
                    raise HTTPException(400, f"not enough technicals marks for trick {trick.acronym}")
                for j, m in trick.technical_marks.items():
                    if j not in technical_marks_per_judge:
                        technical_marks_per_judge[j] = []
                    technical_marks_per_judge[j].append(m)

            technical_marks = []
            for j, marks in technical_marks_per_judge.items():
                _mark = average(marks)
                judge = judges[j]
                weight = dict(config.judge_weights)[judge.level.value]
                technical_marks.append((_mark, weight))

            mark.judges_mark.technical = weight_average(technical_marks)

        #
        # endof technical mark per trick
        #


        technicals = []
        bonuses = []
        bonuses_details = None
        for trick in tricks:
            # calculate the bonus of the run as stated in 7B
            # §6.6.1 Twisted manoeuvres bonus
            if trick.bonus > 0:
                bonuses.append(trick.bonus)

                # 6.6.1 Bonus calculation during AWT and CAT1 events
                # apply the technical mark as a percentage of the bonus
                # if technical mark per trick is sent (reserved for AWT and CAT.1)
                if trick.technical_mark is not None:
                    if bonuses_details is None:
                        bonuses_details = []
                    bonuses_details.append(trick.bonus * trick.technical_mark / 10)

            # as stated in 7B §
            # If more than 5 manoeuvres are flown twisted, the extra manoeuvres will not
            # be scored and their coefficients not taken into account for the
            # determination of the average coefficient.
            technicals.append(trick.technical_coefficient)

        bonuses_total = sum(bonuses)
        if bonuses_details is not None:
            bonuses_details_total = sum(bonuses_details)
            if bonuses_total != bonuses_details_total:
                mark.notes.append(f"Initial total bonus of {bonuses_total}% has been lowered by technical mark to {round(bonuses_details_total,3)}%")
                bonuses_total = bonuses_details_total

        # calculate the technicity of the run as stated in 7B
        # §"6.3.1.1 Technicity in Solo"
        # The technicity is a difficulty coefficient calculated as the average
        # of the 3 highest coefficient manoeuvres flown during the run.
        # add 3 virtual 1.0 tricks to ensure to make an average over 3 tricks
        technicals = sorted(technicals, reverse=True) + [1.0,1.0,1.0]
        mark.technicity = average(technicals[0:3])

        # calculate the bonus of the run as stated in 7B
        # §6.6.1 Twisted manoeuvres bonus
        # -> it is implied that the bonus is the sum of the bonuses limited to 5,3or 2
        #    minus the malus (the malus is applied right after to avoid confusion
        #    we don't want to include the malus in the bonus percentage that is exported to the result site
        mark.bonus_percentage = bonuses_total

        mark_percentage = dict(config.mark_percentages)[self.type.value]
        mark.technical = mark.technicity * mark.judges_mark.technical * mark_percentage.technical / 100
        if len(tricks) > 0:
            mark.choreography = mark.judges_mark.choreography * mark_percentage.choreography / 100
        mark.landing = mark.judges_mark.landing * mark_percentage.landing / 100

        if self.type == CompetitionType.synchro:
            mark.synchro = mark.judges_mark.synchro * mark_percentage.synchro / 100

        mark.bonus = (mark.technical + mark.choreography) * (mark.bonus_percentage - mark.malus) / 100

        mark.score = mark.technical + mark.choreography + mark.landing + mark.synchro + mark.bonus

        # remove warning deduction points
        # 0.5 per warnings
        # §7.2.2 in 7B
        if len(mark.warnings) > 0:
            penalty = (len(mark.warnings) + previous_warnings) * config.warning
            mark.notes.append(f"final mark has been lowered by {penalty} because of warning")
            mark.score -= penalty
        if mark.score < 0:
            mark.score = 0

        return mark
