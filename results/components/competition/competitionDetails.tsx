/* eslint-disable no-unused-vars */
import classNames from "classnames";
import { Fragment, useState } from "react";

import { components } from "@/types";
import { capitalise } from "@/utils/capitalise";

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
  const [showRunDetails, setShowRunDetails] = useState(
    runsResults.map((run) => run.results.map(() => false))
  );

  const toggleRun = (index: number) => {
    const newShowRuns = [...showRuns];
    newShowRuns[index] = !newShowRuns[index];
    setShowRuns(newShowRuns);
  };

  const toggleRunDetails = (runIndex: number, resultIndex: number) => {
    const newShowRunDetails = [...showRunDetails];
    newShowRunDetails[runIndex][resultIndex] =
      !newShowRunDetails[runIndex][resultIndex];
    setShowRunDetails(newShowRunDetails);
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
          className="col-span-full w-3/4 rounded-xl border-[1px] bg-awt-dark-50 px-2 py-2 pb-2 lg:col-span-3 lg:w-full"
        />

        <section
          className={classNames(
            "flex w-full flex-col rounded-xl border-[1px] bg-awt-dark-50 py-2 lg:col-span-7 lg:col-start-4"
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
                    <h4 className="col-span-3 col-start-1 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
                      Pilot
                    </h4>
                    <h4 className="col-span-5 col-start-4 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
                      Judge&apos;s Marks
                    </h4>
                    <h4 className="col-span-2 col-start-9 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
                      Bonus
                    </h4>
                    <h4 className="col-span-2 col-start-11 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
                      Score
                    </h4>
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
                            <header
                              // eslint-disable-next-line jsx-a11y/no-noninteractive-element-to-interactive-role
                              role="button"
                              tabIndex={0}
                              className={classNames(
                                "col-span-3 col-start-1 flex cursor-pointer items-baseline border-[1px] py-2 pl-1"
                              )}
                              onClick={() =>
                                toggleRunDetails(runIndex, resultIndex)
                              }
                              onKeyDown={({ key }) =>
                                key === "Enter" &&
                                toggleRunDetails(runIndex, resultIndex)
                              }
                            >
                              <h4 className="my-auto text-left">
                                {pilot ? pilot.name : "No name record"}
                              </h4>
                              <ChevronIcon
                                className={classNames(
                                  "my-auto ml-1 h-2 w-2",
                                  !showRunDetails[runIndex][resultIndex] &&
                                    "-rotate-90"
                                )}
                              />
                            </header>
                            <div className="col-span-5 grid grid-cols-3 text-center">
                              <p className="border-[1px] py-2">
                                {technicalJudge?.toFixed(3)}
                              </p>
                              <p className="border-[1px] py-2">
                                {choreographyJudge?.toFixed(3)}
                              </p>
                              <p className="border-[1px] py-2">
                                {landingJudge?.toFixed(3)}
                              </p>
                              {run.type === "synchro" && (
                                <p className="border-[1px] py-2">
                                  {synchroJudge?.toFixed(3)}
                                </p>
                              )}
                            </div>

                            <div className="col-span-2 flex justify-center border-[1px] py-2">
                              <WarningIcon
                                className={classNames(
                                  "fill-orange-600",
                                  (warnings?.length || 0) > 0
                                    ? "opacity-100"
                                    : "opacity-0"
                                )}
                              />
                              <p className="px-2">{bonusPercentage}%</p>
                              <ThumbDownIcon
                                className={classNames(
                                  "fill-red-600",
                                  (malus || 0) > 0 ? "opacity-100" : "opacity-0"
                                )}
                              />
                            </div>

                            <p className="col-span-2 border-[1px] py-2 text-center">
                              {roundedScore}
                            </p>

                            {showRunDetails[runIndex][resultIndex] && (
                              <>
                                <h5>Tricks</h5>
                                {tricks.map((trick, trickIndex) => {
                                  const {} = trick;
                                  return (
                                    <Fragment key={trickIndex}>
                                      <p>{capitalise(trick.name)}</p>
                                    </Fragment>
                                  );
                                })}
                                <h5>Details</h5>
                                <h5>Technicity</h5>
                                <p>{technicity?.toFixed(3)}</p>
                                <h5>Malus</h5>
                                <p>{malus}%</p>
                                <h5>Final Technical</h5>
                                <p>{technicalFinal?.toFixed(3)}</p>
                                <h5>Final Choreography</h5>
                                <p>{choreographyFinal?.toFixed(3)}</p>
                                <h5>Final Landing</h5>
                                <p>{landingFinal?.toFixed(3)}</p>

                                {run.type === "synchro" && (
                                  <>
                                    <h5>Final Synchro</h5>
                                    <p>{synchroFinal?.toFixed(3)}</p>
                                  </>
                                )}
                                <h5>Final Bonus</h5>
                                <p>{bonus?.toFixed(3)}</p>

                                {(warnings?.length || 0) > 0 && (
                                  <>
                                    <h5>Warnings</h5>

                                    {warnings?.map((warning, warningIndex) => {
                                      return (
                                        <Fragment key={warningIndex}>
                                          <h5>Warning</h5>
                                          <p>{warning}</p>
                                        </Fragment>
                                      );
                                    })}
                                  </>
                                )}
                                {(notes?.length || 0) > 0 && (
                                  <>
                                    <h5>Notes</h5>

                                    {notes?.map((note, noteIndex) => {
                                      return (
                                        <Fragment key={noteIndex}>
                                          <h5>Note</h5>
                                          <p>{note}</p>
                                        </Fragment>
                                      );
                                    })}
                                  </>
                                )}
                              </>
                            )}
                          </Fragment>
                        );
                      })}
                  </article>
                )}
              </Fragment>
            );
          })}
        </section>
        <CompetitionJudges
          judges={judges}
          className="w-2/3 rounded-xl border-[1px] bg-awt-dark-50 px-2 py-2 pb-2 lg:col-span-2 lg:w-full"
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
                                  <p left="Tricks" />
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
                                  <p left="Details" />
                                  <p
                                    left="Technicity"
                                    right={`${technicity?.toFixed(3)}`}
                                  />
                                  <p left="Malus" right={`${malus}%`} />
                                  <p
                                    left="Final Technical"
                                    right={`${technicalFinal?.toFixed(3)}`}
                                  />
                                  <p
                                    left="Final Choreography"
                                    right={`${choreographyFinal?.toFixed(3)}`}
                                  />
                                  <p
                                    left="Final Landing"
                                    right={`${landingFinal?.toFixed(3)}`}
                                  />
                                  {run.type === "synchro" && (
                                    <p
                                      left="Final Synchro"
                                      right={`${synchroFinal?.toFixed(3)}`}
                                    />
                                  )}
                                  <p
                                    left="Final Bonus"
                                    right={`${bonus?.toFixed(3)}`}
                                  />
                                  {(warnings?.length || 0) > 0 && (
                                    <>
                                      <p left="Warnings" />
                                      {warnings?.map(
                                        (warning, warningIndex) => {
                                          return (
                                            <p
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
                                      <p left="Notes" />
                                      {notes?.map((note, noteIndex) => {
                                        return (
                                          <p
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
