import logging
from pydantic import BaseModel, Field, validator, AnyHttpUrl
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
from models.runs import Run, RunState, RunExport
from models.tricks import Trick
from models.flights import Flight, FlightNew
from models.marks import JudgeMark, FinalMark
from models.competition_configs import CompetitionConfig
from models.results import RunResults, CompetitionResults, CompetitionPilotResults, RunResultSummary, CompetitionResultsExport

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
    image: Optional[AnyHttpUrl]
    seasons: List[str]

    class Config:
        json_encoders = {ObjectId: str}


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
    image: Optional[AnyHttpUrl]
    seasons: List[str]

    class Config:
        json_encoders = {ObjectId: str}

class CompetitionPublicExportWithResults(CompetitionPublicExport):
    results: CompetitionResultsExport
    pilots: List[Pilot]
    teams: List[TeamExport]
    judges: List[Judge]

class CompetitionNew(BaseModel):
    name: str = Field(..., min_len=1)
    code: Optional[str] = Field(regex='^[a-z][a-z0-9-]*[a-z0-9]')
    start_date: date
    end_date: date
    location: str = Field(..., min_len=1)
    published: bool
    type: CompetitionType
    image: Optional[str]
    seasons: List[str] = Field([])


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
    deleted: Optional[datetime]
    image: Optional[str]
    seasons: List[str] = Field([])

    class Config:
        allow_population_by_field_name = True
        arbitrary_types_allowed = True
        json_encoders = {ObjectId: str}
        schema_extra = {
            "example": {
            }
        }

    async def check(self):
        if self.type == CompetitionType.solo:
            for id in self.pilots:
                pilot = await Pilot.get(id)
                if pilot is None:
                    raise HTTPException(400, f"Pilot '{id}' is unknown, only known pilots can take part of a competition")

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
            return f"{settings.SERVER_HOST}/files/{self.image}"
        return None

    async def export(self, cache:dict={}) -> CompetitionExport:

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
            repeatable_tricks.append(await Trick.get(trick, cache=cache))

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
            seasons = self.seasons,
        )

    async def export_public(self) -> CompetitionPublicExport:
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
            seasons = self.seasons,
        )

    async def export_public_with_results(self, cache:dict = {}) -> CompetitionPublicExportWithResults:
        if not self.published:
            raise HTTPException(404, f"Competition {self.code} not published")

        results = await self.results()
        for result in results.runs_results:
            for r in result.results:
                r.marks = []

        comp = await self.export()
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
            seasons = comp.seasons,
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

    async def new_run(self, pilots_to_qualify: int = 0):
        if self.state != CompetitionState.open:
            raise HTTPException(400, "Competition must be 'open' to create a new run")


        pilots = []
        teams = []
        if self.type == CompetitionType.solo:
            pilots = self.pilots
        else:
            teams = self.teams

        if len(self.runs) > 0: # take the list of pilots from the previous run
            # TODO order by the overall ranking after the previous run
            if self.type == CompetitionType.solo:
                pilots = self.runs[-1].pilots
            else:
                teams = self.runs[-1].teams
        else: #  first run to be added, use the list of pilots of the competition
            if self.type == CompetitionType.solo:
                # TODO order by the CIVL ranking
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
            flights=[]
        )
        self.runs.append(run)
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

        flights = []
        all_published=True
        for flight in run.flights:
            if not flight.published:
                all_published=False
                if published_only:
                  continue
            flights.append(flight)

        flights.sort(key=lambda e: e.final_marks.score)

        return RunResults(
            results = flights,
            type = self.type,
            final = (run.state == RunState.closed) and all_published
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

    async def flight_convert(self, id, flight: FlightNew) -> Flight:
        tricks = []
        errors = []
        for trick_name in flight.tricks:
            trick = await Trick.get_unique_trick(trick_name,
                solo  = (self.type==CompetitionType.solo),
                synchro = (self.type==CompetitionType.synchro),
            )
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

    async def flight_save(self, run_i: int, id, flight: FlightNew, save: bool=False, published: bool=False) -> FinalMark:
        run = await self.run_get(run_i)

        if self.type == CompetitionType.solo:
            if int(id) not in run.pilots:
                raise HTTPException(400, f"Pilot #{id} does not participate in the run number #{run_i} of the comp ({self.name})")
            if int(id) not in self.pilots:
                raise HTTPException(400, f"Pilot #{id} does not participate in this comp ({self.name})")

        if self.type == CompetitionType.synchro:
            if id not in run.teams:
                raise HTTPException(400, f"Team #{id} does not participate in the run number #{run_i} of the comp ({self.name})")
            if id not in self.teams:
                raise HTTPException(400, f"Team #{id} does not participate in this comp ({self.name})")

        new_flight = await self.flight_convert(id, flight)
        mark = await self.calculate_score(flight=new_flight, run_i=run_i)
        if not save:
            return mark

        new_flight.final_marks = mark
        new_flight.published = published

        for i, f in enumerate(self.runs[run_i].flights):
            if (self.type == CompetitionType.solo and f.pilot == new_flight.pilot) or (self.type == CompetitionType.synchro and f.team == new_flight.team):
                self.runs[run_i].flights[i] = new_flight
                await self.save()
                return mark

        self.runs[run_i].flights.append(new_flight)
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

            for j, result in enumerate(run_result.results):

                run_result_summary = RunResultSummary(
                    rank = len(run_result.results)-j,
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

                    overall[result.pilot].score += result.final_marks.score
                    overall[result.pilot].result_per_run.append(run_result_summary)

                if self.type == CompetitionType.synchro:
                    if result.team not in overall:
                        overall[result.team] = CompetitionPilotResults(
                            pilot=0,
                            team=result.team,
                            score=0,
                            result_per_run=[]
                        )

                    overall[result.team].score += result.final_marks.score
                    overall[result.team].result_per_run.append(run_result_summary)

        overall_results = list(overall.values())
        overall_results.sort(key=lambda e: e.score)

        return CompetitionResults(
            final = final,
            type = self.type,
            overall_results = overall_results[::-1],
            runs_results = runs_results,
        )

    
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
    async def get(id: str, deleted: bool = False):
        search = {"$or": [{"_id": id}, {"code": id}]}
        if not deleted:
            search['deleted'] = None
        competition = await collection.find_one(search)
        if competition is None:
            raise HTTPException(404, f"Competition {id} not found")
        return Competition.parse_obj(competition)

    @staticmethod
    async def getall(season: str = None):
        competitions = []
        for competition in await collection.find({"deleted": None}, sort=[("name", pymongo.ASCENDING)]).to_list(1000):
            competition = Competition.parse_obj(competition)
            if season is None or season in competition.seasons:
                competitions.append(competition)
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






    async def calculate_score(self, flight: Flight, run_i: int = -1) -> FinalMark:
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

        if (len(technicals) == 0 or len(choreographies) == 0 or len(landings) == 0 or (self.type == CompetitionType.synchro and  len(synchros) == 0)):
            raise HTTPException(400, f"not enough marks")

        mark.judges_mark.technical = weight_average(technicals)
        mark.judges_mark.choreography = weight_average(choreographies)
        mark.judges_mark.landing = weight_average(landings)
        if self.type == CompetitionType.synchro:
            mark.judges_mark.synchro = weight_average(synchros)
        #
        # endof calculating the weight average of the judges marks
        #

        #
        # ignore trick with bonus higher than the maximum bonus tricks allowed
        #
        tricks = [] # the list of tricks that will be used to calculate the scores
        n_bonuses = {}
        i = 0
        for trick in flight.tricks:
            i += 1
            ignoring = False
            for bonus_type in trick.bonus_types:
                if bonus_type not in n_bonuses:
                    n_bonuses[bonus_type] = 0

                n_bonuses[bonus_type] += 1

                max = dict(config.max_bonus_per_run)[bonus_type]

                if n_bonuses[bonus_type] > max:
                    log.warning(f"Ignoring trick #{i} ({trick}) because already {max} tricks have been flown")
                    mark.notes.append(f"trick number #{i} ({trick.name}) has been ignored because more than {max} {bonus_type} tricks have been flown")
                    ignoring = True

            if not ignoring:
                tricks.append(trick)
        #
        # endof ignore tricks max_bonus_per_run
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

        if len(self.runs) > 0 and run_i > 0:
            trick_i = 0
            for trick in tricks: # for each trick detect repetition before
                trick_i += 1
                if trick.base_trick in repeatable_tricks:
                    continue
                # loop over all previous runs
                for i in range(len(self.runs)):
                    if i >= run_i:
                        break
                    r = self.runs[i]
                    broke = False
                    for f in r.flights:
                        if (self.type == CompetitionType.solo and flight.pilot != f.pilot) or (self.type == CompetitionType.synchro and flight.team != f.team):
                            continue
                        for t in f.tricks:
                            if t.base_trick == trick.base_trick and t.uniqueness == trick.uniqueness:
                                log.info(f"base_trick={trick.base_trick}({t.base_trick}) -- uniqueness={trick.uniqueness} ({t.uniqueness})")
                                mark.malus += config.malus_repetition
                                mark.notes.append(f"trick number #{trick_i} ({trick.name}) has already been performed in a previous run. Adding a {config.malus_repetition}% malus.")
                                broke = True
                                break
                    if broke:
                        break

        trick_i = 0
        for trick in tricks: # for each trick detect repetition before
            trick_i += 1
            if trick.base_trick in repeatable_tricks:
                continue
            t_i = 0
            for t in tricks:
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


        technicals = []
        bonuses = []
        for trick in tricks:
            # calculate the bonus of the run as stated in 7B
            # §6.6.1 Twisted manoeuvres bonus
            if trick.bonus > 0:
                bonuses.append(trick.bonus)

            # as stated in 7B §
            # If more than 5 manoeuvres are flown twisted, the extra manoeuvres will not
            # be scored and their coefficients not taken into account for the
            # determination of the average coefficient.
            technicals.append(trick.technical_coefficient)

        # calculate the technicity of the run as stated in 7B 
        # §"6.3.1.1 Technicity in Solo"
        # The technicity is a difficulty coefficient calculated as the average
        # of the 3 highest coefficient manoeuvres flown during the run.
        technicals = sorted(technicals, reverse=True)
        mark.technicity = average(technicals[0:3])

        # calculate the bonus of the run as stated in 7B
        # §6.6.1 Twisted manoeuvres bonus
        # -> it is implied that the bonus is the sum of the bonuses limited to 5,3or 2
        #    minus the malus
        mark.bonus_percentage = sum(bonuses) - mark.malus

        mark_percentage = dict(config.mark_percentages)[self.type.value]
        mark.technical = mark.technicity * mark.judges_mark.technical * mark_percentage.technical / 100
        if len(tricks) > 0:
            mark.choreography = mark.judges_mark.choreography * mark_percentage.choreography / 100 
        mark.landing = mark.judges_mark.landing * mark_percentage.landing / 100

        if self.type == CompetitionType.synchro:
            mark.synchro = mark.judges_mark.synchro * mark_percentage.synchro / 100

        mark.bonus = (mark.technical + mark.choreography) * mark.bonus_percentage / 100

        mark.score = mark.technical + mark.choreography + mark.landing + mark.synchro + mark.bonus

        # remove warning deduction points
        # 0.5 per warnings
        # §7.2.2 in 7B
        mark.score -= ((len(mark.warnings) - previous_warnings)* config.warning)
        if mark.score < 0:
            mark.score = 0

        return mark
