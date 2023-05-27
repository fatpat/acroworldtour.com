// import classNames from "classnames";
import classNames from "classnames";
import Link from "next/link";

import { components } from "@/types";

interface Props {
  results: components["schemas"]["CompetitionPilotResultsExport"][];
}

const CompetitionOverallResults = ({ results }: Props) => (
  <table className="mt-4">
    <thead>
      <tr className="text-left">
        <th scope="col">Rank</th>
        <th scope="col">Pilot</th>
        {/* <th scope="col">Run #</th> */}
        {/* <th scope="col">Run Rank</th> */}
        {/* <th scope="col">Run Score</th> */}
        <th scope="col">Score</th>
      </tr>
    </thead>
    <tbody>
      {results.map((result, resultIndex) => {
        const {
          pilot,
          score: overallScore,
          result_per_run: runResults,
        } = result;

        if (!pilot) return;

        const formattedResults = [];

        if (!runResults.length) {
          formattedResults.push({
            rank: resultIndex + 1,
            pilot: pilot.name,
            score: overallScore.toFixed(3),
            runNumber: null,
            runRank: null,
            runScore: null,
          });
        } else {
          runResults.map((runResult, runIndex) => {
            const firstIndex = runIndex === 0;

            formattedResults.push({
              rank: firstIndex ? resultIndex + 1 : null,
              pilot: firstIndex ? pilot.name : null,
              score: firstIndex ? overallScore.toFixed(3) : null,
              runNumber: runIndex + 1,
              runRank: runResult.rank,
              runScore: runResult.score.toFixed(3),
            });
          });
        }

        return formattedResults.map((formatedResult, index) => {
          return (
            <tr
              key={`${pilot.name}-${index}`}
              className={classNames(
                "text-left",
                resultIndex % 2 === 0 ? "!bg-white" : "!bg-awt-dark-50"
              )}
            >
              <td>{formatedResult.rank}</td>
              <td>
                {formatedResult.pilot && (
                  <Link
                    key={pilot.name}
                    title={`See ${pilot.name}'s profile`}
                    href={`/pilots/${pilot.civlid}/${pilot.name
                      .toLowerCase()
                      .replace(/\s/g, "-")}`}
                  >
                    {pilot.name}
                  </Link>
                )}
              </td>
              {/* <td className="text-center font-extralight italic">
                {formatedResult.runNumber}
              </td>
              <td className="text-center font-extralight italic">
                {formatedResult.runRank}
              </td>
              <td className="text-center font-extralight italic">
                {formatedResult.runScore}
              </td> */}
              <td className="text-right">{formatedResult.score}</td>
            </tr>
          );
        });
      })}
    </tbody>
  </table>
);

export default CompetitionOverallResults;
