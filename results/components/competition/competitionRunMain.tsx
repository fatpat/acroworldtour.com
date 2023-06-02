import classNames from "classnames";
import { Fragment, useState } from "react";

import { components } from "@/types";

import { ChevronIcon } from "../ui/icons";

interface Props {
  results: components["schemas"]["FlightExport"][];
  type: string;
  className?: string;
}

// TODO: Simplify/extract this component, group markup in flex headers

const capitalise = (string: string) =>
  string.charAt(0).toUpperCase() + string.slice(1);

const CompetitionRunMain = ({ results, type, className }: Props) => {
  const [showDetails, setShowDetails] = useState(results.map(() => false));

  const toggleDetails = (index: number) => {
    const newShowDetails = [...showDetails];
    newShowDetails[index] = !newShowDetails[index];
    setShowDetails(newShowDetails);
  };

  results.sort(
    (a, b) => (b.final_marks?.score || 0) - (a.final_marks?.score || 0)
  );

  return (
    <article className={className}>
      <h4 className="col-span-2 col-start-1 border-awt-dark-500 bg-awt-dark-900 py-3 text-white">
        Rank
      </h4>
      <h4 className="col-span-7 border-x-[1px] border-awt-dark-500 bg-awt-dark-900 py-3 text-white">
        Pilot
      </h4>
      <h4 className="col-span-3 border-awt-dark-500 bg-awt-dark-900 py-3 text-white">
        Score
      </h4>
      {results.map((result, resultIndex) => {
        const {
          pilot,
          final_marks,
          tricks,
        } = result;

        const roundedScore =
          final_marks?.score?.toFixed(3) ?? "No score record";

        const {
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

        const rank = resultIndex + 1;

        return (
          <Fragment key={resultIndex}>
            <p className="col-span-2 col-start-1 py-3 text-center">{rank}</p>
            <button
              title="Click to open/close run details"
              className={classNames(
                "col-span-7 flex cursor-pointer items-baseline border-x-[1px] py-3 pl-1"
              )}
              onClick={() => toggleDetails(resultIndex)}
              onKeyDown={({ key }) =>
                key === "Enter" && toggleDetails(resultIndex)
              }
            >
              <h4 className="my-auto text-left">
                {pilot ? pilot.name : "No name record"}
                {["🥇", "🥈", "🥉"][rank - 1]}
              </h4>
              <ChevronIcon
                className={classNames(
                  "my-auto ml-1 h-2 w-2",
                  !showDetails[resultIndex] && "-rotate-90"
                )}
              />
            </button>

            <p className="col-span-3 py-3 text-center">{roundedScore}</p>

            {showDetails[resultIndex] && (
              <>
                <h4 className="col-span-full col-start-1 bg-awt-dark-700 py-3 text-white">
                  Judge&apos;s Marks
                </h4>
                <h5
                  className={classNames(
                    "col-start-1 pt-2",
                    type === "synchro" ? "col-span-3" : "col-span-4"
                  )}
                >
                  Technical
                </h5>
                <h5 className={classNames("col-span-4 border-x-[1px] pt-2")}>
                  Choreography
                </h5>
                <h5
                  className={classNames(
                    "pt-2",
                    type === "synchro" ? "col-span-2" : "col-span-4"
                  )}
                >
                  Landing
                </h5>
                {type === "synchro" && (
                  <h5 className="col-span-3 border-l-[1px] border-t-[1px] pt-2">
                    Synchro
                  </h5>
                )}

                <p
                  className={classNames(
                    "py-1 text-center",
                    type === "synchro" ? "col-span-3" : "col-span-4"
                  )}
                >
                  {technicalJudge?.toFixed(3)}
                </p>
                <p
                  className={classNames(
                    "col-span-4 border-x-[1px] py-1 text-center"
                  )}
                >
                  {choreographyJudge?.toFixed(3)}
                </p>
                <p
                  className={classNames(
                    "py-1 text-center",
                    type === "synchro" ? "col-span-2" : "col-span-4"
                  )}
                >
                  {landingJudge?.toFixed(3)}
                </p>
                {type === "synchro" && (
                  <>
                    <p className="col-span-3 border-x-[1px] py-1 text-center">
                      {synchroJudge?.toFixed(3)}
                    </p>
                  </>
                )}
                <h4 className="col-span-4 col-start-1 bg-awt-dark-700 py-3 text-white">
                  Technicity
                </h4>
                <h4 className="col-span-4 border-x-[1px] border-awt-dark-500 bg-awt-dark-700 py-3 text-white">
                  Bonus
                </h4>
                <h4 className="col-span-4 bg-awt-dark-700 py-3 text-white">
                  Malus
                </h4>

                <p className="col-span-4 col-start-1 py-3 text-center">
                  {technicity?.toFixed(3)}
                </p>
                <p className="col-span-4 flex flex-col items-center justify-center border-x-[1px] py-3">
                  {bonusPercentage}%{(warnings?.length || 0) > 0 && " ⚠️"}
                  {(malus || 0) > 0 && "🔻"}
                </p>
                <p className="col-span-4 py-3 text-center">{malus}%</p>

                <h4 className="col-span-full col-start-1 bg-awt-dark-700 py-3 text-white">
                  Tricks
                </h4>
                <ul className="col-span-full col-start-1 pb-4 pl-4 pt-2">
                  {tricks.map((trick, trickIndex) => (
                    <li key={trickIndex}>
                      <p className="list-item list-inside list-disc pt-2 capitalize">
                        {trick.name}
                      </p>
                    </li>
                  ))}
                </ul>

                <h4 className="col-span-full col-start-1 bg-awt-dark-700 py-3 text-white">
                  Final Marks
                </h4>
                <h5
                  className={classNames(
                    "col-start-1 pt-2",
                    type === "synchro" ? "col-span-3" : "col-span-4"
                  )}
                >
                  Technical
                </h5>
                <h5 className={classNames("col-span-4 border-x-[1px] pt-2")}>
                  Choreography
                </h5>
                <h5
                  className={classNames(
                    "pt-2",
                    type === "synchro" ? "col-span-2" : "col-span-4"
                  )}
                >
                  Landing
                </h5>
                {type === "synchro" && (
                  <h5 className="col-span-3 border-l-[1px] border-t-[1px] pt-2">
                    Synchro
                  </h5>
                )}

                <p
                  className={classNames(
                    "py-1 text-center",
                    type === "synchro" ? "col-span-3" : "col-span-4"
                  )}
                >
                  {technicalFinal?.toFixed(3)}
                </p>
                <p
                  className={classNames(
                    "col-span-4 border-x-[1px] py-1 text-center"
                  )}
                >
                  {choreographyFinal?.toFixed(3)}
                </p>
                <p
                  className={classNames(
                    "py-1 text-center",
                    type === "synchro" ? "col-span-2" : "col-span-4"
                  )}
                >
                  {landingFinal?.toFixed(3)}
                </p>
                {type === "synchro" && (
                  <>
                    <p className="col-span-3 border-x-[1px] py-1 text-center">
                      {synchroFinal?.toFixed(3)}
                    </p>
                  </>
                )}

                <h5 className="col-span-full col-start-1 border-t-[1px] pt-2">
                  Bonus
                </h5>

                <p className="col-span-full col-start-1 py-1 text-center">
                  {(bonus || 0) > 0 ? `${bonus}` : "NIL"}
                </p>

                {(warnings?.length || 0) > 0 && (
                  <>
                    <h5 className="col-span-full col-start-1 border-t-[1px] py-2">
                      Warnings
                    </h5>
                    <ul className="col-span-full col-start-1 py-1 text-center">
                      {warnings?.map((warning, warningIndex) => {
                        return (
                          <li key={warningIndex}>
                            <p className="py-1 pl-6 pr-2 text-left -indent-5">
                              ⚠️ {capitalise(warning)}
                            </p>
                          </li>
                        );
                      })}
                    </ul>
                  </>
                )}

                {(notes?.length || 0) > 0 && (
                  <>
                    <h5 className="col-span-full col-start-1 border-t-[1px] py-2">
                      Notes
                    </h5>
                    <ul className="col-span-full col-start-1 py-1 text-center">
                      {notes?.map((note, noteIndex) => {
                        return (
                          <li key={noteIndex}>
                            <p className="py-1 pl-6 pr-2 text-left -indent-5">
                              📝 {capitalise(note)}
                            </p>
                          </li>
                        );
                      })}
                    </ul>
                  </>
                )}

                <button
                  title="Close run details"
                  className="col-span-full col-start-1 border-y-[1px] bg-white py-3"
                  onClick={() => toggleDetails(resultIndex)}
                  onKeyDown={({ key }) =>
                    key === "Enter" && toggleDetails(resultIndex)
                  }
                >
                  <ChevronIcon className="mx-auto h-3 w-full rotate-180" />
                </button>
              </>
            )}
          </Fragment>
        );
      })}
    </article>
  );
};

export default CompetitionRunMain;
