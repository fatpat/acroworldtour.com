import classNames from "classnames";
import Link from "next/link";

import PilotCard from "@/components/pilot/pilotCard";
import { components } from "@/types";

interface Props {
  trick: components["schemas"]["Trick"];
}

const TrickCard = ({ trick }: Props) => {
  const { _id, name } = trick;
  const urlName = name.toLowerCase().replace(/\s/g, "-");
  return (
    <div>
      <figcaption className="px-2">
        <h3>{name}</h3>
      </figcaption>
    </div>
  );
};

export default TrickCard;
