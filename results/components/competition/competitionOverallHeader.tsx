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
      <p className="col-span-2 col-start-1">{rank}</p>

      <header
        role="button"
        tabIndex={0}
        className={classNames("col-span-8 flex cursor-pointer items-baseline")}
        onClick={() => setShowMore(!showMore)}
        onKeyDown={({ key }) => key === "Enter" && setShowMore(!showMore)}
      >
        <h5>{pilot?.name || "Pilot Unknown"}</h5>
        <ChevronIcon
          className={classNames("ml-2 h-3 w-auto", !showMore && "-rotate-90")}
        />
      </header>

      <p className="col-span-2">{roundedScore}</p>

      {showMore && (
        <>
          <h6 className="col-span-2 col-start-3">Run #</h6>
          <h6 className="col-span-3 col-start-5">Rank</h6>
          <h6 className="col-span-3 col-start-8">Score</h6>

          {runs.map((run, index) => (
            <CompetitionOverallRuns key={index} run={run} index={index} />
          ))}
        </>
      )}
    </>
  );
};

export default CompetitionOverallHeader;
