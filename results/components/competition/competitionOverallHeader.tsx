import classNames from "classnames";
import { useState } from "react";

import { components } from "@/types";

import { ChevronIcon } from "../ui/icons";
import CompetitionOverallRuns from "./competitionOverallRuns";

interface Props {
  result: components["schemas"]["CompetitionPilotResultsExport"];
  rank: number;
}

const CompetitionOverallHeader = ({ result, rank }: Props) => {
  const [showMore, setShowMore] = useState(false);
  const { pilot, score, result_per_run: runs } = result;
  const roundedScore = score.toFixed(3);

  return (
    <>
      <p className="col-span-2 col-start-1 border-[1px] pl-2 py-2">{rank}</p>

      <header
        role="button"
        tabIndex={0}
        className={classNames(
          "col-span-8 flex cursor-pointer items-baseline border-[1px] py-2"
        )}
        onClick={() => setShowMore(!showMore)}
        onKeyDown={({ key }) => key === "Enter" && setShowMore(!showMore)}
      >
        <h5 className="pl-2 text-left my-auto">{pilot?.name || "Pilot Unknown"}</h5>
        <ChevronIcon
          className={classNames("ml-1 h-2 w-2 my-auto", !showMore && "-rotate-90")}
        />
      </header>

      <p className="col-span-2 border-[1px] text-center py-2">{roundedScore}</p>

      {showMore && (
        <>
          <h6 className="col-span-2 col-start-3 bg-awt-dark-500 text-white">
            Run#
          </h6>
          <h6 className="col-span-3 col-start-5 bg-awt-dark-500 text-white">
            Rank
          </h6>
          <h6 className="col-span-3 col-start-8 bg-awt-dark-500 text-white">
            Score
          </h6>

          {runs.map((run, index) => (
            <CompetitionOverallRuns key={index} run={run} index={index} />
          ))}
          <div className="col-start-3 col-span-8 h-8" />
        </>
      )}
    </>
  );
};

export default CompetitionOverallHeader;
