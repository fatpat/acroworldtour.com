/* eslint-disable no-unused-vars */
import classNames from "classnames";
import { Fragment, useState } from "react";

import { components } from "@/types";

import { ChevronIcon, ThumbDownIcon, WarningIcon } from "../ui/icons";
import CompetitionJudges from "./competitionJudges";
import CompetitionOverallResults from "./competitionOverallResults";
import CompetitionSummary from "./competitionSummary";

interface Props {
  competition: components["schemas"]["CompetitionPublicExportWithResults"];
}

const CompetitionDetails = ({ competition }: Props) => {
  const { name, results, judges } = competition;

  const overallResults = results.overall_results;
  overallResults.sort((a, b) => b.score - a.score);

  const runsResults = results.runs_results;

  const [showOverall, setShowOverall] = useState(false);
  const [showRuns, setShowRuns] = useState(runsResults.map(() => false));

  const toggleRun = (index: number) => {
    const newShowRuns = [...showRuns];
    newShowRuns[index] = !newShowRuns[index];
    setShowRuns(newShowRuns);
  };

  return (
    <>
      <h2>{name}</h2>
      <div
        className={classNames(
          "mt-4 grid w-full grid-cols-1 items-start justify-items-center gap-4",
          "lg:grid-cols-12"
        )}
      >
        <CompetitionSummary
          competition={competition}
          className="col-span-full w-3/4 rounded-xl border-[1px] bg-awt-dark-50 px-2 pb-2 lg:col-span-3 lg:w-full"
        />

        <section
          className={classNames(
            "flex flex-col rounded-xl border-[1px] bg-awt-dark-50 lg:col-span-7 lg:col-start-4 lg:w-full",
            showOverall ? "w-full" : "w-3/4"
          )}
        >
          <button
            tabIndex={0}
            className={classNames(
              "col-span-full flex cursor-pointer items-baseline justify-center"
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
          </button>

          {showOverall && (
            <CompetitionOverallResults
              results={overallResults}
              className="mt-4 grid grid-cols-12"
            />
          )}

          {runsResults.map((run, runIndex) => {
            const runNumber = runIndex + 1;

            return (
              <Fragment key={runIndex}>
                <header
                  role="button"
                  tabIndex={0}
                  className={classNames(
                    "col-span-full flex cursor-pointer items-baseline justify-center"
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
                  <article className="mt-4 grid grid-cols-12 border-[1px]">
                    <h4 className="col-span-5 col-start-1 bg-awt-dark-900 text-white">
                      Pilot
                    </h4>
                    <h4 className="col-span-3 col-start-6 bg-awt-dark-900 text-white">
                      Judge&apos;s Marks
                    </h4>
                    <h4 className="col-span-2 col-start-9 bg-awt-dark-900 text-white">
                      Bonus
                    </h4>
                    <h4 className="col-span-2 col-start-11 bg-awt-dark-900 text-white">
                      Score
                    </h4>

                    {/* {run.results.map((result, runIndex) => (
        <RunOverallHeader key={runIndex} result={result} rank={runIndex + 1} />
      ))} */}
                  </article>
                )}
              </Fragment>
            );
          })}
        </section>
        <CompetitionJudges
          judges={judges}
          className="w-2/3 rounded-xl border-[1px] bg-awt-dark-50 px-2 pb-2 lg:col-span-2 lg:w-full"
        />
      </div>
    </>
  );
};

export default CompetitionDetails;

{
  /* {
              showRuns[runIndex] && (
                  <table className="w-full origin-top">
                    <thead>
                      <tr>

                      </tr>
                    </thead>
                    <tbody>
                      {run.results
                        .sort(
                          (a, b) =>
                            (b.final_marks?.score || 0) -
                            (a.final_marks?.score || 0)
                        )
                        .map((result, resultIndex) => {
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
                                  <p>
                                    {technicalJudge?.toFixed(3)}
                                    &nbsp;/&nbsp;
                                    {choreographyJudge?.toFixed(3)}
                                    &nbsp;/&nbsp;
                                    {landingJudge?.toFixed(3)}
                                    {run.type === "synchro" && (
                                      <>
                                        &nbsp;/&nbsp;
                                        {synchroJudge?.toFixed(3)}
                                      </>
                                    )}
                                    {(warnings?.length || 0) > 0 && (
                                      <>
                                        &nbsp;/&nbsp;
                                        <WarningIcon />
                                      </>
                                    )}
                                  </p>
                                </td>
                                <td className="text-right">
                                  <p>{bonusPercentage}%</p>
                                  {(malus || 0) > 0 && <ThumbDownIcon />}
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
                                          <small>
                                            {capitalise(trick.name)}
                                          </small>
                                        </td>
                                      </tr>
                                    );
                                  })}
                                  <TrPrimaryTitle left="Details" />
                                  <TrDuo
                                    left="Technicity"
                                    right={`${technicity?.toFixed(3)}`}
                                  />
                                  <TrDuo left="Malus" right={`${malus}%`} />
                                  <TrDuo
                                    left="Final Technical"
                                    right={`${technicalFinal?.toFixed(3)}`}
                                  />
                                  <TrDuo
                                    left="Final Choreography"
                                    right={`${choreographyFinal?.toFixed(3)}`}
                                  />
                                  <TrDuo
                                    left="Final Landing"
                                    right={`${landingFinal?.toFixed(3)}`}
                                  />
                                  {run.type === "synchro" && (
                                    <TrDuo
                                      left="Final Synchro"
                                      right={`${synchroFinal?.toFixed(3)}`}
                                    />
                                  )}
                                  <TrDuo
                                    left="Final Bonus"
                                    right={`${bonus?.toFixed(3)}`}
                                  />
                                  {(warnings?.length || 0) > 0 && (
                                    <>
                                      <TrSecondaryTitle left="Warnings" />
                                      {warnings?.map(
                                        (warning, warningIndex) => {
                                          return (
                                            <TrDuo
                                              key={warningIndex}
                                              left="Warning"
                                              right={warning}
                                            />
                                          );
                                        }
                                      )}
                                    </>
                                  )}
                                  {(notes?.length || 0) > 0 && (
                                    <>
                                      <TrSecondaryTitle left="Notes" />
                                      {notes?.map((note, noteIndex) => {
                                        return (
                                          <TrDuo
                                            key={noteIndex}
                                            left="Note"
                                            right={note}
                                          />
                                        );
                                      })}
                                    </>
                                  )}
                                </>
                              )}
                            </Fragment>
                          );
                        })}
                    </tbody>
                  </table> */
}
