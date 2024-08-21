import classNames from "classnames";
import { useState } from "react";

import { components } from "@/types";

import { ChevronIcon } from "../ui/icons";

import JudgeCard from "../judge/judgeCard";

interface Props {
  judges: components["schemas"]["Judge"][];
  className?: string;
}

const CompetitionJudges = ({ judges, className }: Props) => {
  const [hidden, setHidden] = useState(true);

  return (
    <section className={className}>
      <button
        className="flex flex-wrap justify-center gap-4 cursor-pointer w-full"
        onClick={() => setHidden(!hidden)}
        onKeyDown={({ key }) => key === "Enter" && setHidden(!hidden)}
      >
        <h3>Judges</h3>
        <ChevronIcon
          className={classNames(
            "my-auto ml-1 h-2 w-2",
            hidden && "-rotate-90",
          )}
        />
      </button>
      <article className={classNames(
            "flex flex-wrap justify-center gap-4",
            hidden && "landscape:hidden",
          )}>
        {judges.map((judge) => (
          <JudgeCard key={judge.name} judge={judge} small />
        ))}
      </article>
    </section>
  )
};

export default CompetitionJudges;
