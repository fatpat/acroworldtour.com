import Link from "next/link";

import { components } from "@/types";

interface Props {
  season: components["schemas"]["SeasonExport"];
}

const SeasonCard = ({ season }: Props) => {
  const {
    code,
    name,
    image,
    number_of_pilots: numberOfPilots,
    year,
    type,
  } = season;
  return (
    <Link
      key={code}
      href={`/seasons/${code}`}
      className="w-full max-w-lg hover:-translate-y-2 hover:invert"
    >
      <article
        style={{ backgroundImage: `url(${image})` }}
        className="card flex h-48 flex-col justify-between p-4"
      >
        <hgroup>
          <h3>{name}</h3>
          <h4>{year}</h4>
        </hgroup>
        <div className="flex justify-between">
          <small className="capitalize">{type}</small>
          <small>{`${numberOfPilots} pilots`}</small>
        </div>
      </article>
    </Link>
  );
};

export default SeasonCard;
