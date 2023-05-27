import classNames from "classnames";

import { components } from "@/types";

import JudgeCard from "../judge/judgeCard";

interface Props {
  judges: components["schemas"]["Judge"][];
}

const CompetitionJudges = ({ judges }: Props) => (
  <section
    className={classNames("flex flex-col rounded-xl bg-awt-dark-50", "lg:p-4")}
  >
    <h3>Judges</h3>
    <article
      className={classNames("mt-4 flex w-full flex-wrap justify-evenly gap-4")}
    >
      {judges.map((judge) => (
        <JudgeCard key={judge.name} judge={judge} />
      ))}
    </article>
  </section>
);

export default CompetitionJudges;
