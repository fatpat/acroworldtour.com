/* eslint-disable no-unused-vars */
import classNames from "classnames";
import Image from "next/image";
import { Fragment, useState } from "react";

import { components } from "@/types";
import { capitalise } from "@/utils/capitalise";

import JudgeCard from "../judge/judgeCard";
import { ChevronIcon } from "../ui/icons";

interface Props {
  competition: components["schemas"]["CompetitionPublicExportWithResults"];
}

interface trProps {
  left: string;
  right?: string;
}

const TrDuo = ({ left, right }: trProps) => (
  <tr>
    <td className="py-2 pl-8">
      <small>{left}</small>
    </td>
    <td className="py-2 pl-8 text-right">
      <small>{right}</small>
    </td>
  </tr>
);

const TrPrimaryTitle = ({ left }: trProps) => (
  <tr className="text-left font-semibold">
    <td colSpan={2} className="bg-awt-dark-800 py-2 pl-6 text-white">
      <p>{left}</p>
    </td>
  </tr>
);

const TrSecondaryTitle = ({ left }: trProps) => (
  <tr>
    <td colSpan={2} className="bg-awt-dark-600 py-2 pl-6 text-white">
      <small>{left}</small>
    </td>
  </tr>
);

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
            "flex min-w-max flex-col gap-2 rounded-xl bg-awt-dark-50",
            "lg:p-4"
          )}
        >
          {image && (
            <Image
              src={image}
              alt="Competition Image"
              width={512}
              height={0}
              className="my-2 h-auto w-full rounded-xl"
            />
          )}
          <p>{`Type: ${capitalise(type)}`}</p>
          <p>{`Location: ${location}`}</p>
          <p>{`Pilots: ${numberOfPilots}`}</p>
          <p>{`Start Date: ${startDate}`}</p>
          <p>{`End Date: ${endDate}`}</p>
        </section>

        <section
          className={classNames(
            "flex w-full flex-col rounded-xl bg-awt-dark-50"
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
                    if (!pilot) return;
                    return (
                      <tr key={pilot.name}>
                        <td>
                          <p>{pilot.name}</p>
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
                        <th className="bg-awt-dark-800 text-left text-white">
                          Pilot
                        </th>
                        <th className="bg-awt-dark-800 text-right text-white">
                          Score
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {run.results.map((result, resultIndex) => {
                        const {
                          pilot,
                          final_marks,
                          tricks,
                          // marks,
                          // warnings,
                        } = result;

                        const roundedScore =
                          final_marks?.score?.toFixed(3) ?? "No score record";

                        const {
                          score,
                          technicity,
                          bonus_percentage: bonusPercentage,
                          malus,
                          judges_mark,
                          technical: technicalFinal,
                          choreography: choreographyFinal,
                          landing: landingFinal,
                          synchro: synchroFinal,
                          bonus,
                          warnings,
                          notes,
                        } = final_marks ?? {};

                        const {
                          technical: technicalJudge,
                          choreography: choreographyJudge,
                          landing: landingJudge,
                          synchro: synchroJudge,
                        } = judges_mark ?? {};

                        return (
                          <Fragment key={resultIndex}>
                            <tr>
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
                                <p>{pilot ? pilot.name : "No name record"}</p>
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
                                <TrPrimaryTitle left="Tricks" />
                                {tricks.map((trick, trickIndex) => {
                                  const {} = trick;
                                  return (
                                    <tr key={trickIndex}>
                                      <td colSpan={2} className="py-2 pl-8">
                                        <small>{capitalise(trick.name)}</small>
                                      </td>
                                    </tr>
                                  );
                                })}
                                <TrPrimaryTitle left="Marks" />
                                <TrDuo
                                  left="Technicity"
                                  right={`${technicity}%`}
                                />
                                <TrDuo
                                  left="Bonus Percentage"
                                  right={`${bonusPercentage}%`}
                                />
                                <TrDuo left="Malus" right={`${malus}%`} />
                                <TrSecondaryTitle left="Judge's Marks" />
                                <TrDuo
                                  left="Technical"
                                  right={`${technicalJudge}%`}
                                />
                                <TrDuo
                                  left="Choreography"
                                  right={`${choreographyJudge}%`}
                                />
                                <TrDuo
                                  left="Landing"
                                  right={`${landingJudge}%`}
                                />
                                {run.type === "synchro" && (
                                  <TrDuo
                                    left="Synchro"
                                    right={`${synchroJudge}%`}
                                  />
                                )}
                                <TrSecondaryTitle left="Final Marks" />
                                <TrDuo
                                  left="Technical"
                                  right={`${technicalFinal}%`}
                                />
                                <TrDuo
                                  left="Choreography"
                                  right={`${choreographyFinal}%`}
                                />
                                <TrDuo
                                  left="Landing"
                                  right={`${landingFinal}%`}
                                />
                                {run.type === "synchro" && (
                                  <TrDuo
                                    left="Synchro"
                                    right={`${synchroFinal}%`}
                                  />
                                )}
                                <TrDuo left="Bonus" right={`${bonus}%`} />
                                <TrSecondaryTitle
                                  left={
                                    warnings?.length ?? 0 > 0
                                      ? "Warnings"
                                      : "No Warnings"
                                  }
                                />
                                {warnings &&
                                  warnings.map((warning, warningIndex) => {
                                    return (
                                      <TrDuo
                                        key={warningIndex}
                                        left="Warning"
                                        right={warning}
                                      />
                                    );
                                  })}
                              </>
                            )}
                          </Fragment>
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
            "flex flex-col rounded-xl bg-awt-dark-50",
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
