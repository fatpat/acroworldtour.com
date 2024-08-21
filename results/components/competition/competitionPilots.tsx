import classNames from "classnames";
import { useState } from "react";

import { components } from "@/types";

import { ChevronIcon } from "../ui/icons";

import PilotCard from "../pilot/pilotCard";

interface Props {
  pilots: components["schemas"]["Pilot"][];
  className?: string;
}

const CompetitionPilots = ({ pilots, className }: Props) => {
  const [hidden, setHidden] = useState(true);

  return(
    <section className={className}>
      <button
        className="flex flex-wrap justify-center gap-4 cursor-pointer w-full"
        onClick={() => setHidden(!hidden)}
        onKeyDown={({ key }) => key === "Enter" && setHidden(!hidden)}
      >
        <h3>Pilots</h3>
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
        {pilots.map((pilot) => (
          <PilotCard key={pilot.name} pilot={pilot} small />
        ))}
      </article>
    </section>
  )
};

export default CompetitionPilots;
