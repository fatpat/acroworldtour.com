import { Fragment } from "react";

import { components } from "@/types";

type CompetitionResult = components["schemas"]["CompetitionPilotResultsExport"];

interface Props {
  results: {
    [key: string]: CompetitionResult[] | undefined;
  };
  pilotId: number;
  className?: string;
}

type PilotResult = {
  competition: string;
  rank: number;
  score: number;
};

const SeasonOverallPilotResults = ({ results, pilotId }: Props) => {
  const pilotResults: PilotResult[] = [];

  for (const [competition, competitionResults] of Object.entries(results)) {
    competitionResults?.forEach((result, index) => {
      const { pilot, score } = result;
      if (pilot?.civlid === pilotId) {
        pilotResults.push({ competition, rank: index + 1, score });
      }
    });
  }

  return (
    <Fragment>
      {pilotResults.map((result, index) => {
        const { competition, rank, score } = result;
        const roundedScore = score.toFixed(3);
        return (
          <Fragment key={index}>
            <p className="col-span-2 col-start-1 border-[1px] bg-awt-dark-500 py-2 pl-2 text-center text-white">
              {rank} at
            </p>
            <h6 className="col-span-8 flex items-center border-[1px] bg-awt-dark-500 p-2 text-left capitalize text-white">
              {competition}
            </h6>
            <p className="col-span-2 col-start-11 border-[1px] bg-awt-dark-500 py-2 text-center text-white">
              {roundedScore}
            </p>
          </Fragment>
        );
      })}
    </Fragment>
  );
};

export default SeasonOverallPilotResults;
