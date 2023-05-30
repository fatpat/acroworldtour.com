import { components } from "@/types";

import CompetitionOverallHeader from "./competitionOverallHeader";

interface Props {
  results: components["schemas"]["CompetitionPilotResultsExport"][];
  className?: string;
}

const CompetitionOverallResults = ({ results, className }: Props) => {
  return (
    <article className={className}>
      <h4 className="col-span-2 col-start-1 bg-awt-dark-900 text-white border-[1px] border-awt-dark-500 py-1">Rank</h4>
      <h4 className="col-span-8 col-start-3 bg-awt-dark-900 text-white border-[1px] border-awt-dark-500 py-1">Pilot</h4>
      <h4 className="col-span-2 col-start-11 bg-awt-dark-900 text-white border-[1px] border-awt-dark-500 py-1">Score</h4>

      {results.map((result, index) => (
        <CompetitionOverallHeader key={index} result={result} rank={index + 1} />
      ))}
    </article>
  );
};

export default CompetitionOverallResults;
