import classNames from "classnames";
import Image from "next/image";
import { useState } from "react";

import { components } from "@/types";
import { capitalise } from "@/utils/capitalise";

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
  const [showRuns, setShowRuns] = useState(runsResults.map(() => false));
  const [showRunDetails, setShowRunDetails] = useState(
    runsResults.map((run) => run.results.map(() => false))
  );

  const toggleRun = (index: number) => {
    const newShowRuns = [...showRuns];
    newShowRuns[index] = !newShowRuns[index];
    setShowRuns(newShowRuns);
  };

  const toggleRunDetails = (runIndex: number, resultIndex: number) => {
    const newShowDetails = [...showRunDetails];
    newShowDetails[runIndex][resultIndex] =
      !newShowDetails[runIndex][resultIndex];
    setShowRunDetails(newShowDetails);
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
            "flex w-full flex-col rounded-xl bg-awtgrey-50"
          )}
        >
          <article>
            <header
              role="button"
              tabIndex={0}
              className={classNames(
                "flex cursor-pointer items-baseline pl-4 pt-4"
              )}
              onClick={() => setShowOverall(!showOverall)}
              onKeyDown={({ key }) =>
                key === "Enter" && setShowOverall(!showOverall)
              }
            >
              <h3>Overall Results</h3>
              <ChevronIcon
                className={classNames(
                  "ml-2 h-3 w-auto",
                  !showOverall && "-rotate-90"
                )}
              />
            </header>
            {showOverall && (
              <table className="w-full origin-top">
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
                        <td>
                          <p>{pilot!.name}</p>
                        </td>
                        <td className="text-right">
                          <p>{roundedScore}</p>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </article>

          {runsResults.map((run, runIndex) => {
            const runNumber = runIndex + 1;

            return (
              <article key={runNumber}>
                <header
                  role="button"
                  tabIndex={0}
                  className={classNames(
                    "flex cursor-pointer items-baseline pl-4 pt-4"
                  )}
                  onClick={() => toggleRun(runIndex)}
                  onKeyDown={({ key }) =>
                    key === "Enter" && toggleRun(runIndex)
                  }
                >
                  <h3>{`Run ${runNumber}`}</h3>
                  <ChevronIcon
                    className={classNames(
                      "ml-2 h-3 w-auto",
                      !showRuns[runIndex] && "-rotate-90"
                    )}
                  />
                </header>
                {showRuns[runIndex] && (
                  <table className="w-full origin-top">
                    <thead>
                      <tr>
                        <th className="text-left">Pilot</th>
                        <th className="text-right">Score</th>
                      </tr>
                    </thead>
                    <tbody>
                      {run.results.map((result, resultIndex) => {
                        const {
                          pilot,
                          final_marks,
                          tricks,
                          // marks
                        } = result;
                        const roundedScore = final_marks!.score.toFixed(3);
                        return (
                          <>
                            <tr key={resultIndex}>
                              <td
                                // eslint-disable-next-line jsx-a11y/no-noninteractive-element-to-interactive-role
                                role="button"
                                tabIndex={0}
                                className={classNames(
                                  "flex cursor-pointer items-baseline"
                                )}
                                onClick={() =>
                                  toggleRunDetails(runIndex, resultIndex)
                                }
                                onKeyDown={({ key }) =>
                                  key === "Enter" &&
                                  toggleRunDetails(runIndex, resultIndex)
                                }
                              >
                                <p>{pilot?.name}</p>
                                <ChevronIcon
                                  className={classNames(
                                    "ml-2 h-2 w-auto",
                                    !showRunDetails[runIndex][resultIndex] &&
                                      "-rotate-90"
                                  )}
                                />
                              </td>
                              <td className="text-right">
                                <p>{roundedScore}</p>
                              </td>
                            </tr>

                            {showRunDetails[runIndex][resultIndex] && (
                              <>
                                <th className="py-2 pl-8 text-left">
                                  <p className="font-semibold">Tricks</p>
                                </th>
                                {tricks.map((trick, trickIndex) => (
                                  <tr key={trickIndex}>
                                    <td colSpan={2} className="py-2 pl-8">
                                      <small>{capitalise(trick.name)}</small>
                                    </td>
                                  </tr>
                                ))}
                              </>
                            )}
                          </>
                        );
                      })}
                    </tbody>
                  </table>
                )}
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
