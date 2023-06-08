import { components } from "@/types";

import CompetitionOverallHeader from "./competitionOverallHeader";

interface Props {
  results: components["schemas"]["CompetitionPilotResultsExport"][];
  type: components["schemas"]["CompetitionType"];
  className?: string;
}

const CompetitionOverallResults = ({ results, type, className }: Props) => {
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

      {results.map((result, index) => (
        <CompetitionOverallHeader
          key={index}
          result={result}
          rank={index + 1}
          type={type}
        />
      ))}
    </article>
  );
};

export default CompetitionOverallResults;
