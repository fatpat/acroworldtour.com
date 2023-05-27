/* eslint-disable no-unused-vars */
import classNames from "classnames";
import Image from "next/image";
import Link from "next/link";
import { Fragment, useState } from "react";

import { components } from "@/types";
import { capitalise } from "@/utils/capitalise";

import JudgeCard from "../judge/judgeCard";
import { ChevronIcon, ThumbDownIcon, WarningIcon } from "../ui/icons";
import CompetitionSummary from "./competitionSummary";

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
        <CompetitionSummary competition={competition} />

        <section
          className={classNames(
            "flex w-full flex-col rounded-xl bg-awt-dark-50"
          )}
        >
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
            <table className="w-full table-auto origin-top">
              <thead>
                <tr>
                  <th className="text-left align-top" rowSpan={2}>
                    Rank
                  </th>
                  <th className="text-left align-top" rowSpan={2}>
                    Pilot
                  </th>
                  <th className="text-center align-top" colSpan={3}>
                    Runs Results
                  </th>
                  <th className="text-right align-top" rowSpan={2}>
                    Score
                  </th>
                </tr>
                <tr>
                  <th className="text-center">Run</th>
                  <th className="text-center">Rank</th>
                  <th className="text-center">Score</th>
                </tr>
              </thead>
              <tbody>
                {overallResults
                  .sort((a, b) => b.score - a.score)
                  .map((result, index) => {
                    const { pilot, score, result_per_run } = result;
                    if (!pilot) return;
                    let data = [];
                    if (result_per_run.length == 0) {
                      data.push({
                        rank: index + 1,
                        pilot: pilot.name,
                        score: score.toFixed(3),
                        run_number: null,
                        run_rank: null,
                        run_score: null,
                      });
                    } else {
                      result_per_run.map((r, i) => {
                        data.push({
                          rank: i == 0 ? index + 1 : null,
                          pilot: i == 0 ? pilot.name : null,
                          score: i == 0 ? score.toFixed(3) : null,
                          run_number: i + 1,
                          run_rank: r.rank,
                          run_score: r.score.toFixed(3),
                        });
                      });
                    }
                    return data.map((r, i) => {
                      return (
                        <tr
                          key={`${pilot.name}-${i}`}
                          className={
                            index % 2 == 0 ? "!bg-white" : "!bg-awt-dark-50"
                          }
                        >
                          <td className="text-left">
                            <p>{r["rank"]}</p>
                          </td>
                          <td className="text-left">
                            {r["pilot"] && (
                              <p>
                                <Link
                                  key={pilot.name}
                                  title={`See ${pilot.name}'s profile`}
                                  href={`/pilots/${pilot.civlid}/${pilot.name
                                    .toLowerCase()
                                    .replace(/\s/g, "-")}`}
                                >
                                  {pilot.name}
                                </Link>
                              </p>
                            )}
                          </td>
                          <td className="text-center font-extralight italic">
                            <p>{r["run_number"]}</p>
                          </td>
                          <td className="text-center font-extralight italic">
                            <p>{r["run_rank"]}</p>
                          </td>
                          <td className="text-center font-extralight italic">
                            <p>{r["run_score"]}</p>
                          </td>
                          <td className="text-right">
                            <p>{r["score"]}</p>
                          </td>
                        </tr>
                      );
                    });
                    {
                      /*
                    return ({
                      data.map((r, i) => {
console.log(r, i)
                        return (
                          <tr key={`${pilot.name}-${r.run}`}">

                            <td className="text-right">
                              <p>Run {i+1}</p>
                            </td>
                            <td className="text-right">
                              <p>P{r[3]}</p>
                            </td>
                            <td className="text-right">
                              <p>{r[5]}</p>
                            </td>
                            <td className="text-right">
                              <p>{r[2]}</p>
                            </td>
                          </tr>
                          </>
                      })
                    });
*/
                    }
                  })}
              </tbody>
            </table>
          )}

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
                          Judge&apos;s Marks
                        </th>
                        <th className="bg-awt-dark-800 text-right text-white">
                          Bonus
                        </th>
                        <th className="bg-awt-dark-800 text-right text-white">
                          Score
                        </th>
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
