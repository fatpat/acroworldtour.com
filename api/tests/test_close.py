import pytest
from unittest.mock import AsyncMock, patch
from fastapi import HTTPException

from models.competitions import Competition, CompetitionState, CompetitionType
from models.runs import Run, RunState
from models.flights import Flight
from models.pilots import Pilot
from models.teams import Team


def make_flight(pilot, published):
    return Flight.model_construct(pilot=pilot, team=None, published=published)


def make_run(state, pilots, flights, teams=[]):
    return Run.model_construct(state=state, pilots=pilots, teams=teams, flights=flights)


def make_comp(state, runs, type=CompetitionType.solo):
    return Competition.model_construct(type=type, state=state, runs=runs)


async def fake_get(id, **kwargs):
    if id in (404, "404"):
        raise HTTPException(404, "not found")
    return type("Participant", (), {"name": f"name of {id}"})()


@pytest.fixture
def anyio_backend():
    return "asyncio"


@pytest.fixture(autouse=True)
def no_db():
    with patch.object(Competition, "save", new=AsyncMock()) as save, \
         patch.object(Pilot, "get", new=fake_get), \
         patch.object(Team, "get", new=fake_get):
        yield save


@pytest.mark.anyio
async def test_run_close_with_all_flights_published():
    comp = make_comp(CompetitionState.open, [make_run(RunState.open, [1, 2], [make_flight(1, True), make_flight(2, True)])])
    await comp.run_close(0)
    assert comp.runs[0].state == RunState.closed


@pytest.mark.anyio
async def test_run_close_with_missing_flight():
    comp = make_comp(CompetitionState.open, [make_run(RunState.open, [1, 2], [make_flight(1, True)])])
    with pytest.raises(HTTPException) as e:
        await comp.run_close(0)
    assert e.value.status_code == 400
    assert "not saved (name of 2)" in e.value.detail
    assert comp.runs[0].state == RunState.open


@pytest.mark.anyio
async def test_run_close_with_unpublished_flight():
    comp = make_comp(CompetitionState.open, [make_run(RunState.open, [1, 2], [make_flight(1, True), make_flight(2, False)])])
    with pytest.raises(HTTPException) as e:
        await comp.run_close(0)
    assert e.value.status_code == 400
    assert "not published (name of 2)" in e.value.detail
    assert comp.runs[0].state == RunState.open


@pytest.mark.anyio
async def test_run_close_with_unpublished_team_flight():
    flights = [Flight.model_construct(pilot=None, team="t1", published=True), Flight.model_construct(pilot=None, team="t2", published=False)]
    comp = make_comp(CompetitionState.open, [make_run(RunState.open, [], flights, teams=["t1", "t2"])], type=CompetitionType.synchro)
    with pytest.raises(HTTPException) as e:
        await comp.run_close(0)
    assert "not published (name of t2)" in e.value.detail


@pytest.mark.anyio
async def test_run_close_with_unknown_pilot_falls_back_to_id():
    comp = make_comp(CompetitionState.open, [make_run(RunState.open, [1, 404], [make_flight(1, True)])])
    with pytest.raises(HTTPException) as e:
        await comp.run_close(0)
    assert e.value.status_code == 400
    assert "not saved (404)" in e.value.detail


@pytest.mark.anyio
async def test_comp_close_with_all_runs_closed():
    comp = make_comp(CompetitionState.open, [make_run(RunState.closed, [], []), make_run(RunState.closed, [], [])])
    await comp.close()
    assert comp.state == CompetitionState.closed


@pytest.mark.anyio
async def test_comp_close_with_run_not_closed():
    comp = make_comp(CompetitionState.open, [make_run(RunState.closed, [], []), make_run(RunState.open, [], []), make_run(RunState.init, [], [])])
    with pytest.raises(HTTPException) as e:
        await comp.close()
    assert e.value.status_code == 400
    assert "run(s) #1, #2" in e.value.detail
    assert comp.state == CompetitionState.open
