from models.seasons import Season, SeasonResults, SeasonResult
from models.pilots import Pilot, GenderEnum
from models.teams import TeamExport


def make_result(civlid, gender, score):
    return SeasonResult.model_construct(pilot=Pilot.model_construct(civlid=civlid, gender=gender), team=None, score=score)


def make_overall(genders):
    results = [make_result(i, gender, 100 - i) for i, gender in enumerate(genders)]
    return SeasonResults.model_construct(type="overall", results=results)


def test_women_ranking_with_3_women():
    overall = make_overall([GenderEnum.man, GenderEnum.woman, GenderEnum.man, GenderEnum.woman, GenderEnum.woman])
    women = Season.women_results(overall)
    assert women.type == "women"
    assert [r.pilot.civlid for r in women.results] == [1, 3, 4]
    assert [r.score for r in women.results] == [99, 97, 96]


def test_no_women_ranking_with_less_than_3_women():
    overall = make_overall([GenderEnum.woman, GenderEnum.man, GenderEnum.woman, GenderEnum.none])
    assert Season.women_results(overall) is None


def test_no_women_ranking_for_synchro():
    results = [SeasonResult.model_construct(pilot=None, team=TeamExport.model_construct(), score=1) for _ in range(5)]
    overall = SeasonResults.model_construct(type="overall", results=results)
    assert Season.women_results(overall) is None
