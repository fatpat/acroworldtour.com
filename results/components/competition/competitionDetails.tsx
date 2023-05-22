import classNames from "classnames";
import Link from "next/link";
import { useState } from "react";

import { components } from "@/types";

import JudgeCard from "../judgeCard";
import { ChevronIcon } from "../ui/icons";

interface Props {
  competition: components["schemas"]["CompetitionPublicExportWithResults"];
}

const CompetitionDetails = ({ competition }: Props) => {
  const [showResults, setShowResults] = useState<boolean>(false);

  const {
    code,
    name,
    judges,
    start_date: startDate,
    end_date: endDate,
    location,
    pilots,
    results,
    state,
    type,
    image,
  } = competition;

  const overallResults = results.overall_results;
  const runsResults = results.runs_results;

  return (
    <div
      style={{ backgroundImage: `url('${image}')` }}
      className={classNames(
        "bg-white/95 bg-cover bg-center bg-no-repeat bg-blend-overlay",
        "flex min-h-screen w-full flex-col items-center"
      )}
    >
      <h2>{name}</h2>
      <div
        className={classNames(
          "mt-4 flex w-full flex-col justify-evenly gap-6 px-4",
          "lg:flex-row "
        )}
      >
        <section
          className={classNames(
            "flex flex-1 flex-col items-center bg-red-200/30 p-4"
          )}
        >
          <header
            className={classNames("flex cursor-pointer items-baseline")}
            onClick={() => setShowResults(!showResults)}
          >
            <h3>Overall Results</h3>
            <ChevronIcon
              className={classNames(
                "ml-2 h-3 w-auto",
                !showResults && "-rotate-90"
              )}
            />
          </header>
          {showResults && (
            <table className="w-11/12">
              <thead>
                <tr className="h-12">
                  <th className="text-left">Pilot</th>
                  <th className="text-right">Score</th>
                </tr>
              </thead>
              <tbody>
                {overallResults.map((result) => {
                  const { pilot, score, result_per_run } = result;
                  const roundedScore = Math.round(score * 100) / 100;
                  return (
                    <tr key={pilot!.name} className="h-12">
                      <td>{pilot!.name}</td>
                      <td className="text-right">{roundedScore}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}

          {runsResults.map( ( run ) => {
            const runNumber = runsResults.indexOf( run ) + 1;
            
            return (
              <>
                <header
                  className={classNames("flex cursor-pointer items-baseline")}
                  onClick={() => setShowResults(!showResults)}
                >
                  <h3>{`Run ${runNumber}`}</h3>
                  <ChevronIcon
                    className={classNames(
                      "ml-2 h-3 w-auto",
                      !showResults && "-rotate-90"
                    )}
                  />
                </header>
                {run.results.map((result) => {
                  return (
                    <p key={result.pilot!.name}>{result.tricks[0].name}</p>
                  );
                })}
              </>
            );
          })}

          {/* <header
            className={classNames("flex cursor-pointer items-baseline")}
            onClick={() => setShowResults(!showResults)}
          >
            <h3>Overall Results</h3>
            <ChevronIcon
              className={classNames(
                "ml-2 h-3 w-auto",
                !showResults && "-rotate-90"
              )}
            />
          </header>
          <table className="w-11/12">
            <thead>
              <tr className="h-12">
                <th className="text-left">Pilot</th>
                <th className="text-right">Score</th>
              </tr>
            </thead>
            <tbody>
              {results.runs_results.map((result) => {
                console.log(result);
                const {} = result;
                const roundedScore = Math.round(score * 100) / 100;
                return (
                  <tr key={pilot!.name} className="h-12">
                    <td>{pilot!.name}</td>
                    <td className="text-right">{roundedScore}</td>
                  </tr>
                );
              })}
            </tbody>
          </table> */}
        </section>
        <section
          className={classNames(
            "flex flex-1 flex-col items-center bg-green-200/30 p-4"
          )}
        >
          <h3>Details</h3>
          <article className={classNames()}></article>
        </section>
        <section
          className={classNames(
            "flex flex-1 flex-col items-center bg-blue-200/30 p-4"
          )}
        >
          <h3>Judges</h3>
          <article
            className={classNames(
              "mt-4 flex w-full flex-wrap justify-evenly gap-4"
            )}
          >
            {judges.map((judge) => (
              <JudgeCard key={judge.name} judge={judge} />
            ))}
          </article>
        </section>
      </div>
    </div>
  );
};

export default CompetitionDetails;
