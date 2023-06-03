import Link from "next/link";
import { Fragment } from "react";

import { components } from "@/types";

interface Props {
  results: components["schemas"]["models__seasons__SeasonResult"][];
  className?: string;
}

const SeasonCategoryResults = ({ results, className }: Props) => (
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
          <p className="col-span-2 col-start-1 border-[1px] py-2 pl-2">
            {rank}
          </p>
          <h5 className="col-span-8 flex cursor-pointer items-baseline border-[1px] py-2">
            {pilot ? (
              <Link href={`/pilots/${pilot.civlid}/${pilot.name}`}>
                {pilot.name}
                {["🥇", "🥈", "🥉"][rank - 1]}
              </Link>
            ) : (
              "Pilot Unknown"
            )}
          </h5>
          <p className="col-span-2 border-[1px] py-2 text-center">
            {roundedScore}
          </p>
        </Fragment>
      );
    })}
  </article>
);

export default SeasonCategoryResults;
