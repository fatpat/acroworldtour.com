import logging
from tempfile import NamedTemporaryFile
from openpyxl import Workbook

from models.competitions import Competition, CompetitionResultsExport, CompetitionType, CompetitionExport, CompetitionResults
from models.results import RunResultsExport
from models.svg_data import SvgData
from controllers.utils import UtilsCtrl

log = logging.getLogger(__name__)

class CompCtrl:
    @staticmethod
    def start():
        Competition.createIndexes()

    @staticmethod
    def comp_to_xlsx(comp:CompetitionResultsExport, type:CompetitionType):
        ret = None
        wb = Workbook()
        ws = wb.active
        ws.title = f"overall {type}"

        if type == CompetitionType.solo:
            ws.cell(column=1, row=1, value="Rank")
            ws.cell(column=2, row=1, value="Number")
            ws.cell(column=3, row=1, value="First Name")
            ws.cell(column=4, row=1, value="Last Name")
            ws.cell(column=5, row=1, value="Nat")
            ws.cell(column=6, row=1, value="Gender")
            ws.cell(column=7, row=1, value="Glider")
            ws.cell(column=8, row=1, value="Sponsor")
            ws.cell(column=9, row=1, value="CIVL ID")
            ws.cell(column=10, row=1, value="Score")

        if type == CompetitionType.synchro:
            ws.cell(column=1, row=1, value="Rank")
            ws.cell(column=2, row=1, value="Team")
            ws.cell(column=3, row=1, value="Number")
            ws.cell(column=4, row=1, value="First Name")
            ws.cell(column=5, row=1, value="Last Name")
            ws.cell(column=6, row=1, value="Nat")
            ws.cell(column=7, row=1, value="Gender")
            ws.cell(column=8, row=1, value="Glider")
            ws.cell(column=9, row=1, value="Sponsor")
            ws.cell(column=10, row=1, value="CIVL ID")
            ws.cell(column=11, row=1, value="Score")

        rank = 0
        row = 1
        for res in comp.overall_results:
            rank += 1
            score = round(res.score, 3)
            if type == CompetitionType.solo:
                row += 1
                ws.cell(column=1, row=row, value=f"{rank}")
                ws.cell(column=2, row=row, value=f"{res.pilot.civlid}")
                ws.cell(column=3, row=row, value=res.pilot.name.split(" ", 1)[0])
                ws.cell(column=4, row=row, value=res.pilot.name.split(" ", 1)[1])
                ws.cell(column=5, row=row, value=res.pilot.country.upper())
                ws.cell(column=6, row=row, value=f"M")
                ws.cell(column=7, row=row, value=f"")
                ws.cell(column=8, row=row, value=f"")
                ws.cell(column=9, row=row, value=f"{res.pilot.civlid}")
                ws.cell(column=10, row=row, value=f"{score}")

            if type == CompetitionType.synchro:
                for i, pilot in enumerate(res.team.pilots):
                    row += 1
                    ws.cell(column=1, row=row, value=f"{rank}")
                    ws.cell(column=2, row=row, value=f"{res.team.name}")
                    ws.cell(column=3, row=row, value=f"{pilot.civlid}")
                    ws.cell(column=4, row=row, value=pilot.name.split(" ", 1)[0])
                    ws.cell(column=5, row=row, value=pilot.name.split(" ", 1)[1])
                    ws.cell(column=6, row=row, value=pilot.country.upper())
                    ws.cell(column=7, row=row, value=f"M")
                    ws.cell(column=8, row=row, value=f"")
                    ws.cell(column=9, row=row, value=f"")
                    ws.cell(column=10, row=row, value=f"{pilot.civlid}")
                    ws.cell(column=11, row=row, value=f"{score}")
                ws.merge_cells(start_row=row-1, start_column=1, end_row=row, end_column=1)
                ws.merge_cells(start_row=row-1, start_column=2, end_row=row, end_column=2)

        with NamedTemporaryFile(suffix=".xlsx", delete=False) as xlsx:
            ret = xlsx.name
            wb.save(filename=ret)

        return ret

    @staticmethod
    def run_to_xlsx(run:RunResultsExport, type:CompetitionType):
        if not run.final:
            raise HTTPException(400, f"Can't export run because it's not marked as final")
        ret = None
        wb = Workbook()
        ws = wb.active
        ws.title = f"run {type}"

        if type == CompetitionType.solo:
            ws.cell(column=1, row=1, value="Rank")
            ws.cell(column=2, row=1, value="Number")
            ws.cell(column=3, row=1, value="First Name")
            ws.cell(column=4, row=1, value="Last Name")
            ws.cell(column=5, row=1, value="Nat")
            ws.cell(column=6, row=1, value="Gender")
            ws.cell(column=7, row=1, value="Glider")
            ws.cell(column=8, row=1, value="Sponsor")
            ws.cell(column=9, row=1, value="CIVL ID")
            ws.cell(column=10, row=1, value="Score")

        if type == CompetitionType.synchro:
            ws.cell(column=1, row=1, value="Rank")
            ws.cell(column=2, row=1, value="Team")
            ws.cell(column=3, row=1, value="Number")
            ws.cell(column=4, row=1, value="First Name")
            ws.cell(column=5, row=1, value="Last Name")
            ws.cell(column=6, row=1, value="Nat")
            ws.cell(column=7, row=1, value="Gender")
            ws.cell(column=8, row=1, value="Glider")
            ws.cell(column=9, row=1, value="Sponsor")
            ws.cell(column=10, row=1, value="CIVL ID")
            ws.cell(column=11, row=1, value="Score")

        rank = 0
        row = 1
        run.results.sort(key=lambda e: -e.final_marks.score)
        for res in run.results:
            rank += 1
            score = round(res.final_marks.score, 3)
            if type == CompetitionType.solo:
                row += 1
                ws.cell(column=1, row=row, value=f"{rank}")
                ws.cell(column=2, row=row, value=f"{res.pilot.civlid}")
                ws.cell(column=3, row=row, value=res.pilot.name.split(" ", 1)[0])
                ws.cell(column=4, row=row, value=res.pilot.name.split(" ", 1)[1])
                ws.cell(column=5, row=row, value=res.pilot.country.upper())
                ws.cell(column=6, row=row, value=f"M")
                ws.cell(column=7, row=row, value=f"")
                ws.cell(column=8, row=row, value=f"")
                ws.cell(column=9, row=row, value=f"{res.pilot.civlid}")
                ws.cell(column=10, row=row, value=f"{score}")

            if type == CompetitionType.synchro:
                for i, pilot in enumerate(res.team.pilots):
                    row += 1
                    ws.cell(column=1, row=row, value=f"{rank}")
                    ws.cell(column=2, row=row, value=f"{res.team.name}")
                    ws.cell(column=3, row=row, value=f"{pilot.civlid}")
                    ws.cell(column=4, row=row, value=pilot.name.split(" ", 1)[0])
                    ws.cell(column=5, row=row, value=pilot.name.split(" ", 1)[1])
                    ws.cell(column=6, row=row, value=pilot.country.upper())
                    ws.cell(column=7, row=row, value=f"M")
                    ws.cell(column=8, row=row, value=f"")
                    ws.cell(column=9, row=row, value=f"")
                    ws.cell(column=10, row=row, value=f"{pilot.civlid}")
                    ws.cell(column=11, row=row, value=f"{score}")
                ws.merge_cells(start_row=row-1, start_column=1, end_row=row, end_column=1)
                ws.merge_cells(start_row=row-1, start_column=2, end_row=row, end_column=2)

        with NamedTemporaryFile(suffix=".xlsx", delete=False) as xlsx:
            ret = xlsx.name
            wb.save(filename=ret)

        return ret

    @staticmethod
    def svg_overall(competition: CompetitionResults):
        results = []
        for rank, result in enumerate(competition.overall_results):
            results.append(SvgData(
                rank=rank+1,
                country=result.pilot.country if competition.type == "solo" else None,
                name=result.pilot.name if competition.type == "solo" else result.team.name,
                score="%.3f" % result.score
            ))

        return UtilsCtrl.svg(results)

    @staticmethod
    def svg_run(compResults: CompetitionResults, run: int):
        compResults = compResults.runs_results[run-1]
        results = []
        compResults.results.sort(key=lambda e: -e.final_marks.score)
        for rank, result in enumerate(compResults.results):
            results.append(SvgData(
                rank=rank+1,
                country=result.pilot.country if compResults.type == "solo" else None,
                name=result.pilot.name if compResults.type == "solo" else result.team.name,
                score="%.3f" % result.final_marks.score
            ))

        return UtilsCtrl.svg(results)
