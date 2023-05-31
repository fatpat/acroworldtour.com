import { components } from "@/types";

interface Props {
  run: components["schemas"]["RunResultSummary"];
  index: number;
}

const CompetitionOverallHeader = ({ run, index }: Props) => {
  const {score, rank} = run

  return (
    <>
      <p className="col-span-2 col-start-3 text-center border-[1px]">{index + 1}</p>
      <p className="col-span-3 text-center border-[1px]">{["🥇", "🥈", "🥉"][rank-1] || rank}</p>
      <p className="col-span-3 text-center border-[1px]">{score}</p>
    </>
  );
};

export default CompetitionOverallHeader;
