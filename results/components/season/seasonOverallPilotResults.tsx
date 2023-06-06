import classNames from "classnames";
import Link from "next/link";
import { Fragment } from "react";

import { components } from "@/types";
import { ordinalSuffixOf } from "@/utils/common";

type CompetitionResult = components["schemas"]["CompetitionPilotResultsExport"];
type CompetitionPublicExport = components["schemas"]["CompetitionPublicExport"];

interface Props {
  results: {
    [key: string]: CompetitionResult[] | undefined;
  };
  pilotId: number;
  competitions: CompetitionPublicExport[];
  className?: string;
}

type PilotResult = {
  competition: string;
  rank: number;
  score: number;
};

const SeasonOverallPilotResults = ({
  results,
  pilotId,
  competitions,
}: Props) => {
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
        const { competition: competitionCode, rank, score } = result;
        const roundedScore = score.toFixed(3);
        const competitionName = competitions.find(
          (c) => c.code == competitionCode,
        )?.name;
        return (
          <Fragment key={index}>
            <Link
              href={`/competitions/${competitionCode}`}
              title={`Competition page for ${competitionName}`}
              className={classNames(
                "col-span-8 col-start-3",
                "flex items-center",
                "border-[1px] bg-awt-dark-300 pl-2",
              )}
            >
              <h6 className="hover:font-bold">
                {ordinalSuffixOf(rank)} at {competitionName}
              </h6>
            </Link>
            <p className="col-span-2 col-start-11 border-[1px] bg-awt-dark-300 py-2 text-center">
              {roundedScore}
            </p>
          </Fragment>
        );
      })}
    </Fragment>
  );
};

export default SeasonOverallPilotResults;
