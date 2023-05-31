import { components } from "@/types";

interface Props {
  run: components["schemas"]["RunResultSummary"];
  index: number;
}

const CompetitionOverallHeader = ({ run, index }: Props) => {
  const { score, rank } = run;

  return (
    <>
      <p className="col-span-2 col-start-3 border-[1px] text-center">
        {index + 1}
      </p>
      <p className="col-span-3 border-[1px] text-center">{rank}</p>
      <p className="col-span-3 border-[1px] text-center">{score}</p>
    </>
  );
};

export default CompetitionOverallHeader;
