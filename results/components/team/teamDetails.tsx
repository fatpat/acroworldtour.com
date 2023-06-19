import classNames from "classnames";

import { components } from "@/types";

import PilotCard from "../pilot/pilotCard";

interface Props {
  team: components["schemas"]["TeamExport"];
}

const TeamDetails = ({ team }: Props) => {
  const { name, pilots } = team;

  return (
    <>
      <h2 className="mb-4">{name}</h2>
      <section
        className={classNames("mt-2 flex flex-wrap justify-center gap-8")}
      >
        {pilots.map((pilot) => (
          <PilotCard key={pilot.civlid} pilot={pilot} />
        ))}
      </section>
    </>
  );
};

export default TeamDetails;
