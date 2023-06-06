import classNames from "classnames";
import { Fragment, useState } from "react";

import { components } from "@/types";

import { ChevronIcon } from "../ui/icons";
import SeasonOverallPilotResults from "./SeasonOverallPilotResults";

type SeasonResult = components["schemas"]["models__seasons__SeasonResult"];
type CompetitionResult = components["schemas"]["CompetitionPilotResultsExport"];

interface Props {
  results: SeasonResult[];
  competitionResults: {
    [key: string]: CompetitionResult[] | undefined;
  };
  className?: string;
}

const SeasonCategoryResults = ({
  results,
  competitionResults,
  className,
}: Props) => {
  const [expandPilot, setExpandPilot] = useState(results.map(() => false));

  const toggleShowMore = (index: number) => {
    const newShowMore = [...expandPilot].fill(false);
    newShowMore[index] = !expandPilot[index];

    setExpandPilot(newShowMore);
  };

  return (
    <article className={className}>
      <h4 className="col-span-2 col-start-1 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
        Rank
      </h4>
      <h4 className="col-span-8 col-start-3 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
        Pilot
      </h4>
      <h4 className="col-span-2 col-start-11 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
        Score
      </h4>

      {results.map((result, index) => {
        const { pilot, score } = result;
        const rank = index + 1;
        const roundedScore = score.toFixed(3);
        return (
          <Fragment key={index}>
            <p className="col-span-2 col-start-1 border-[1px] py-2 pl-2 text-center">
              {rank}
            </p>
            <button
              title="Show/hide score by competition"
              className="col-span-8 flex cursor-pointer items-baseline border-[1px] py-2"
              onClick={() => toggleShowMore(index)}
              onKeyDown={({ key }) => key === "Enter" && toggleShowMore(index)}
            >
              <h5 className="my-auto pl-2 text-left">
                {pilot?.name || "Pilot Unknown"}
                {["🥇", "🥈", "🥉"][rank - 1]}
              </h5>
              <ChevronIcon
                className={classNames(
                  "my-auto ml-1 h-2 w-2",
                  !expandPilot[index] && "-rotate-90",
                )}
              />
            </button>
            <p className="col-span-2 border-[1px] py-2 text-center">
              {roundedScore}
            </p>
            {expandPilot[index] && pilot && competitionResults && (
              <>
                <SeasonOverallPilotResults
                  results={competitionResults}
                  pilotId={pilot.civlid}
                />
                <button
                  className="col-span-full col-start-1 border-y-[1px]"
                  onClick={() => toggleShowMore(index)}
                  onKeyDown={({ key }) =>
                    key === "Enter" && toggleShowMore(index)
                  }
                >
                  <ChevronIcon className="mx-auto my-3 h-3 rotate-180" />
                </button>
              </>
            )}
          </Fragment>
        );
      })}
    </article>
  );
};

export default SeasonCategoryResults;
