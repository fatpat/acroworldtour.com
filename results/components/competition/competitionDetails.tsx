import classNames from "classnames";
import Image from "next/image";
import { useState } from "react";

import { components } from "@/types";

import JudgeCard from "../judge/judgeCard";
import { ChevronIcon } from "../ui/icons";

interface Props {
  competition: components["schemas"]["CompetitionPublicExportWithResults"];
}

const CompetitionDetails = ({ competition }: Props) => {
  const {
    end_date: endDate,
    image,
    judges,
    location,
    name,
    number_of_pilots: numberOfPilots,
    results,
    start_date: startDate,
    type,
  } = competition;

  const overallResults = results.overall_results;
  const runsResults = results.runs_results;

  const [showOverall, setShowOverall] = useState(false);
  const [showRun, setShowRun] = useState(runsResults.map(() => false));

  const toggleRunVisibility = (index: number) => {
    const newShowRuns = [...showRun];
    newShowRuns[index] = !newShowRuns[index];
    setShowRun(newShowRuns);
  };

  return (
    <div
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
            "flex min-w-max flex-col rounded-xl bg-awtgrey-50",
            "lg:p-4"
          )}
        >
          <Image
            src={image!}
            alt="Competition Image"
            width={512}
            height={0}
            className="h-auto w-full"
          />
          <h3>Details</h3>
          <p>{`Type: ${type}`}</p>
          <p>{`Location: ${location}`}</p>
          <p>{`Pilots: ${numberOfPilots}`}</p>
          <p>{`Start Date: ${startDate}`}</p>
          <p>{`End Date: ${endDate}`}</p>
        </section>

        <section
          className={classNames(
            "flex w-full flex-col rounded-xl bg-awtgrey-50",
          )}
        >
          <article>
            <header
              className={classNames("flex cursor-pointer items-baseline p-4")}
              onClick={() => setShowOverall(!showOverall)}
            >
              <h3>Overall Results</h3>
              <ChevronIcon
                className={classNames(
                  "ml-2 h-3 w-auto",
                  !showOverall && "-rotate-90"
                )}
              />
            </header>
            <table
              className={classNames(
                "w-full origin-top",
                !showOverall && "collapse scale-y-0"
              )}
            >
              <thead>
                <tr>
                  <th className="text-left">Pilot</th>
                  <th className="text-right">Score</th>
                </tr>
              </thead>
              <tbody>
                {overallResults.map((result) => {
                  const { pilot, score } = result;
                  const roundedScore = score.toFixed(3);
                  return (
                    <tr key={pilot!.name}>
                      <td>{pilot!.name}</td>
                      <td className="text-right">{roundedScore}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </article>

          {runsResults.map((run, index) => {
            const runNumber = index + 1;

            return (
              <article key={runNumber}>
                <header
                  className={classNames("flex cursor-pointer items-baseline p-4")}
                  onClick={() => toggleRunVisibility(index)}
                >
                  <h3>{`Run ${runNumber}`}</h3>
                  <ChevronIcon
                    className={classNames(
                      "ml-2 h-3 w-auto",
                      !showRun[index] && "-rotate-90"
                    )}
                  />
                </header>
                <table
                  className={classNames(
                    "w-full origin-top",
                    !showRun[index] && "collapse scale-y-0"
                  )}
                >
                  <thead>
                    <tr>
                      <th className="text-left">Pilot</th>
                      <th className="text-right">Score</th>
                    </tr>
                  </thead>
                  <tbody>
                    {run.results.map((result) => {
                      const { pilot, final_marks } = result;
                      const roundedScore = final_marks!.score.toFixed(3);
                      return (
                        <tr key={pilot!.name}>
                          <td>{pilot!.name}</td>
                          <td className="text-right">{roundedScore}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </article>
            );
          })}
        </section>

        <section
          className={classNames(
            "flex flex-col rounded-xl bg-awtgrey-50",
            "lg:p-4"
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
