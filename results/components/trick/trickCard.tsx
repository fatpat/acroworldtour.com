import { components } from "@/types";

interface Props {
  trick: components["schemas"]["Trick-Input"];
}

const TrickCard = ({ trick }: Props) => {
  return (
    <figcaption className="px-2">
      <h3>{trick.name}</h3>
    </figcaption>
  );
};

export default TrickCard;
