import classNames from "classnames";
import Link from "next/link";

import PilotCard from "@/components/pilot/pilotCard";
import { components } from "@/types";

interface Props {
  team: components["schemas"]["TeamExport"];
}

const TeamCard = ({ team }: Props) => {
  const { _id, name, pilots } = team;
  const urlName = name.toLowerCase().replace(/\s/g, "-");
  pilots.map((pilot) => (console.log(pilot)))
  return (
    <div>
      <figcaption className="px-2">
        <h3>{name}</h3>
          {pilots.map((pilot) => (
            <PilotCard key={pilot.civlid} pilot={pilot} />
          ))}
      </figcaption>
    </div>
  );
};

export default TeamCard;
