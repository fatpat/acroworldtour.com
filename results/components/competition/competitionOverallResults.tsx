import { components } from "@/types";

import CompetitionOverallHeader from "./competitionOverallHeader";

interface Props {
  results: components["schemas"]["CompetitionPilotResultsExport"][];
}

const CompetitionOverallResults = ({ results }: Props) => {
  return (
    <article className="col-span-full grid grid-cols-12">
      <h4 className="col-span-2 col-start-1">Rank</h4>
      <h4 className="col-span-8 col-start-3">Pilot</h4>
      <h4 className="col-span-2 col-start-11">Score</h4>

      {results.map((result, index) => (
        <CompetitionOverallHeader key={index} result={result} rank={index + 1} />
      ))}
    </article>
  );
};

export default CompetitionOverallResults;
