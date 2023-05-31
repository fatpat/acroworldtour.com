import classNames from "classnames";
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
      className={classNames("max-w-lg flex-grow")}
    >
      <article
        style={{ backgroundImage: `url(${image})` }}
        className={classNames(
          "flex h-48 flex-col justify-between rounded-xl text-white",
          "bg-black/60 bg-cover bg-center bg-no-repeat p-4 bg-blend-multiply",
          "shadow shadow-awt-dark-400",
          "hover:-translate-y-2 hover:bg-white/90 hover:text-current hover:bg-blend-screen",
          "hover:shadow-md"
        )}
      >
        <hgroup>
          <h3 className="text-left">{name}</h3>
          <h4 className="text-left">{year}</h4>
        </hgroup>
        <div className="flex justify-between">
          <p className="capitalize">{type}</p>
          <p>{`${numberOfPilots} pilots`}</p>
        </div>
      </article>
    </Link>
  );
};

export default SeasonCard;
