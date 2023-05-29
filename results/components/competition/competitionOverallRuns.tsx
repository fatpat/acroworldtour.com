import { components } from "@/types";

interface Props {
  run: components["schemas"]["RunResultSummary"];
  index: number;
}

const CompetitionOverallHeader = ({ run, index }: Props) => {
  const {score, rank} = run

  return (
    <>
      <p className="col-span-2 col-start-3">{index + 1}</p>
      <p className="col-span-3">{rank}</p>
      <p className="col-span-3">{score}</p>
    </>
  );
};

export default CompetitionOverallHeader;
