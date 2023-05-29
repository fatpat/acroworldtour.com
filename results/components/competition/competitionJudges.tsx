import { components } from "@/types";

import JudgeCard from "../judge/judgeCard";

interface Props {
  judges: components["schemas"]["Judge"][];
  className?: string;
}

const CompetitionJudges = ({ judges, className }: Props) => (
  <section className={className}>
    <h3 className="mb-4">Judges</h3>
    {judges.map((judge) => (
      <JudgeCard key={judge.name} judge={judge} />
    ))}
  </section>
);

export default CompetitionJudges;
