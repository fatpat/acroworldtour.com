import logging

from models.seasons import Season, SeasonExport
from models.svg_data import SvgData
from controllers.utils import UtilsCtrl

log = logging.getLogger(__name__)

class SeasonCtrl:
    @staticmethod
    def start():
        Season.createIndexes()

    @staticmethod
    def svg(season: SeasonExport):
        results = []
        for rank, result in enumerate(season.results[0].results):
            results.append(SvgData(
                rank=rank+1,
                country=result.pilot.country if season.type == "solo" else None,
                name=result.pilot.name if season.type == "solo" else result.team.name,
                score="%.3f" % result.score
            ))

        return UtilsCtrl.svg(results)
