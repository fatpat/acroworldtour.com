import PilotCard from "@/components/pilot/pilotCard";
import { components } from "@/types";

interface Props {
  team: components["schemas"]["TeamExport"];
}

const TeamCard = ({ team }: Props) => {
  const { name, pilots } = team;
  // pilots.map((pilot) => console.log(pilot));
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
