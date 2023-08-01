import { components } from "@/types";

import PilotCard from "../pilot/pilotCard";

interface Props {
  pilots: components["schemas"]["Pilot"][];
  className?: string;
}

const CompetitionPilots = ({ pilots, className }: Props) => (
  <section className={className}>
    <h3 className="mb-4">Pilots</h3>
    <article className="flex flex-wrap justify-center gap-4">
      {pilots.map((pilot) => (
        <PilotCard key={pilot.name} pilot={pilot} small />
      ))}
    </article>
  </section>
);

export default CompetitionPilots;
