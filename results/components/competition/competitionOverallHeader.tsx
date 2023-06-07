import classNames from "classnames";
import { useState } from "react";

import { components } from "@/types";

import { ChevronIcon } from "../ui/icons";
import CompetitionOverallRuns from "./competitionOverallRuns";

interface Props {
  result: components["schemas"]["CompetitionPilotResultsExport"];
  type: components["schemas"]["CompetitionType"];
  rank: number;
}

const CompetitionOverallHeader = ({ result, rank, type }: Props) => {
  const [showMore, setShowMore] = useState(false);
  const { pilot, team, score, result_per_run: runs } = result;
  const roundedScore = score.toFixed(3);

  return (
    <>
      <p className="col-span-2 col-start-1 border-[1px] py-2 pl-2 text-center">
        {rank}
      </p>

      <button
        className="col-span-8 flex cursor-pointer items-baseline border-[1px] py-2"
        onClick={() => setShowMore(!showMore)}
        onKeyDown={({ key }) => key === "Enter" && setShowMore(!showMore)}
      >
        <h5 className="my-auto pl-2 text-left">
          {type === "solo" && (pilot?.name || "Unknown Pilot")}
          {type === "synchro" && (team?.name || "Unknown Team")}
          {["🥇", "🥈", "🥉"][rank - 1]}
          {type === "synchro" && team && (
            <ul className="font-normal">
              <li>{team?.pilots[0].name}</li>
              <li>{team?.pilots[1].name}</li>
            </ul>
          )}
        </h5>
        <ChevronIcon
          className={classNames(
            "my-auto ml-1 h-2 w-2",
            !showMore && "-rotate-90",
          )}
        />
      </button>

      <p className="col-span-2 border-[1px] py-2 text-center">{roundedScore}</p>

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
          <div className="col-span-8 col-start-3 h-8" />
        </>
      )}
    </>
  );
};

export default CompetitionOverallHeader;
