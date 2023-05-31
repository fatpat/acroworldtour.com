import classNames from "classnames";
import { Fragment, useState } from "react";

import { components } from "@/types";

import { ChevronIcon } from "../ui/icons";

interface Props {
  results: components["schemas"]["FlightExport"][];
  type: string;
  className?: string;
}

/* TODO: Simplify/extract this component, group markup in flex headers */

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
      <h4 className="col-span-6 col-start-1 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-3 text-white">
        Pilot
      </h4>
      <h4 className="col-span-3 col-start-7 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-3 text-white">
        Bonus
      </h4>
      <h4 className="col-span-3 col-start-10 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-3 text-white">
        Score
      </h4>
      {results.map((result, resultIndex) => {
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
            <button
              title="Click to open/close run details"
              className={classNames(
                "col-span-6 col-start-1 flex cursor-pointer items-baseline border-[1px] py-2 pl-1"
              )}
              onClick={() => toggleDetails(resultIndex)}
              onKeyDown={({ key }) =>
                key === "Enter" && toggleDetails(resultIndex)
              }
            >
              <h4 className="my-auto text-left">
                {pilot ? pilot.name : "No name record"}
              </h4>
              <ChevronIcon
                className={classNames(
                  "my-auto ml-1 h-2 w-2",
                  !showDetails[resultIndex] && "-rotate-90"
                )}
              />
            </button>

            <p className="col-span-3 flex flex-col items-center justify-center border-[1px] py-2">
              {bonusPercentage}%{(warnings?.length || 0) > 0 && " ⚠️"}
              {(malus || 0) > 0 && "🔻"}
            </p>

            <p className="col-span-3 border-[1px] py-2 text-center">
              {roundedScore}
            </p>

            {showDetails[resultIndex] && (
              <>
                <h4 className="col-span-full col-start-1 bg-awt-dark-700 py-3 text-white">
                  Judge&apos;s Marks
                </h4>
                <h5 className="col-span-3 border-[1px] py-2">Technical</h5>
                <h5 className="col-span-4 border-[1px] py-2">Choreography</h5>
                <h5 className="col-span-3 border-[1px] py-2">Landing</h5>
                <h5
                  className={classNames(
                    "col-span-2 border-[1px] bg-awt-dark-100 py-2 text-center",
                    type === "solo" && "text-awt-dark-400"
                  )}
                >
                  Synchro
                </h5>
                <p className="col-span-3 border-[1px] py-1 text-center">
                  {technicalJudge?.toFixed(3)}
                </p>
                <p className="col-span-4 border-[1px] py-1 text-center">
                  {choreographyJudge?.toFixed(3)}
                </p>
                <p className="col-span-3 border-[1px] py-1 text-center">
                  {landingJudge?.toFixed(3)}
                </p>
                <p
                  className={classNames(
                    "col-span-2 border-[1px] py-1 text-center",
                    type === "solo" && "text-awt-dark-400"
                  )}
                >
                  {type === "synchro" ? synchroJudge?.toFixed(3) : "N/A"}
                </p>
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
                  Details
                </h4>
                <h5 className="col-span-6 col-start-1 border-[1px] py-2">
                  Technicity
                </h5>
                <h5 className="col-span-6 border-[1px] py-2">Malus</h5>

                <p className="col-span-6 col-start-1 border-[1px] py-1 text-center">
                  {technicity?.toFixed(3)}
                </p>
                <p className="col-span-6 border-[1px] py-1 text-center">
                  {malus}%
                </p>
                <h5 className="col-span-full col-start-1 bg-awt-dark-500 py-2 text-white">
                  Final Marks
                </h5>
                <h6 className="col-span-4 col-start-1 border-[1px] py-2">
                  Technical
                </h6>
                <h6 className="col-span-4 border-[1px] py-2">Choreography</h6>
                <h6 className="col-span-4 border-[1px] py-2">Landing</h6>
                <p className="col-span-4 col-start-1 border-[1px] py-1 text-center">
                  {technicalFinal?.toFixed(3)}
                </p>
                <p className="col-span-4 border-[1px] py-1 text-center">
                  {choreographyFinal?.toFixed(3)}
                </p>
                <p className="col-span-4 border-[1px] py-1 text-center">
                  {landingFinal?.toFixed(3)}
                </p>

                <h6 className="col-span-6 col-start-1 border-[1px] py-2">
                  Bonus
                </h6>
                <h6
                  className={classNames(
                    "col-span-6 border-[1px] py-2",
                    type === "solo" && "text-awt-dark-400"
                  )}
                >
                  Synchro
                </h6>
                <p className="col-span-6 col-start-1 border-[1px] py-1 text-center">
                  {bonus?.toFixed(3)}
                </p>
                <p
                  className={classNames(
                    "col-span-6 border-[1px] py-1 text-center",
                    type === "solo" && "text-awt-dark-400"
                  )}
                >
                  {type === "synchro" ? synchroFinal?.toFixed(3) : "N/A"}
                </p>
                {(warnings?.length || 0) > 0 && (
                  <>
                    <h5 className="col-span-full col-start-1 border-[1px] py-1">
                      Warnings
                    </h5>

                    <ul className="col-span-full col-start-1 border-[1px] py-1 text-center">
                      {warnings?.map((warning, warningIndex) => {
                        return (
                          <li key={warningIndex}>
                            <p className="py-1">⚠️ {warning}</p>
                          </li>
                        );
                      })}
                    </ul>
                  </>
                )}
                {(notes?.length || 0) > 0 && (
                  <>
                    <h5 className="col-span-full col-start-1 border-[1px] py-1">
                      Notes
                    </h5>
                    <ul className="col-span-full col-start-1 border-[1px] py-1 pl-4">
                      {notes?.map((note, noteIndex) => {
                        return (
                          <li key={noteIndex}>
                            <p className="py-1">📝 {note}</p>
                          </li>
                        );
                      })}
                    </ul>
                  </>
                )}
                <button
                  title="Close run details"
                  className={classNames(
                    "col-span-full col-start-1 bg-white py-3",
                    "hover:bg-awt-accent-600 hover:fill-white"
                  )}
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
