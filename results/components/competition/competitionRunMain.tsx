import classNames from "classnames";
import { Fragment, useState } from "react";

import { components } from "@/types";
import { capitalise } from "@/utils/capitalise";

import { ChevronIcon, ThumbDownIcon, WarningIcon } from "../ui/icons";

interface Props {
  results: components["schemas"]["FlightExport"][];
  type: string;
  className?: string;
}

const CompetitionRunMain = ({ results, type, className }: Props) => {
  const [showDetails, setShowDetails] = useState(results.map(() => false));

  const toggleDetails = (index: number) => {
    const newShowDetails = [...showDetails];
    newShowDetails[index] = !newShowDetails[index];
    setShowDetails(newShowDetails);
  };

  return (
    <article className={className}>
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
      {results
        .sort(
          (a, b) => (b.final_marks?.score || 0) - (a.final_marks?.score || 0)
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
              </header>
              <div className="col-span-5 grid grid-cols-3 text-center">
                <p className="border-[1px] py-2">
                  {technicalJudge?.toFixed(3)}
                </p>
                <p className="border-[1px] py-2">
                  {choreographyJudge?.toFixed(3)}
                </p>
                <p className="border-[1px] py-2">{landingJudge?.toFixed(3)}</p>
                {type === "synchro" && (
                  <p className="border-[1px] py-2">
                    {synchroJudge?.toFixed(3)}
                  </p>
                )}
              </div>

              <div className="col-span-2 flex flex-col items-center justify-center border-[1px] py-2">
                <p>{bonusPercentage}%</p>
                {(warnings?.length || 0) > 0 && (
                  <WarningIcon className="h-4 fill-orange-600" />
                )}
                {(malus || 0) > 0 && (
                  <ThumbDownIcon className="h-4 fill-red-600" />
                )}
              </div>

              <p className="col-span-2 border-[1px] py-2 text-center">
                {roundedScore}
              </p>

              {showDetails[resultIndex] && (
                <>
                  <h5 className="col-span-full col-start-1 border-[1px] border-awt-dark-500 bg-awt-dark-600 py-1 text-white">
                    Tricks
                  </h5>
                  <ul className="col-span-full col-start-1 pl-4">
                    {tricks.map((trick, trickIndex) => (
                      <li
                        key={trickIndex}
                      >
                       <p className="">{capitalise(trick.name)}</p>
                      </li>
                    ))}
                  </ul>
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

                  {type === "synchro" && (
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
  );
};

export default CompetitionRunMain;
