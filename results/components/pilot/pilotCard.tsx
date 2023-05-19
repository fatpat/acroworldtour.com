import classNames from "classnames";
import { alpha3ToAlpha2 } from "i18n-iso-countries";
import Link from "next/link";

import { components } from "@/types";

interface Props {
  pilot: components["schemas"]["Pilot"];
}

const PilotCard = ({ pilot }: Props) => {
  const { civlid, name, photo, country, rank } = pilot;
  const urlName = name.toLowerCase().replace(/\s/g, "-");
  const alpha2country = alpha3ToAlpha2(country.toUpperCase()).toLowerCase();
  return (
    <Link
      key={civlid}
      title={`See ${name}'s profile`}
      href={`/pilots/${civlid}/${urlName}`}
      className="flex flex-col rounded-xl pb-4 hover:-translate-y-2 hover:shadow-xl w-full sm:w-48"
    >
      <figure
        style={{ backgroundImage: `url('${photo}')` }}
        className="pilot-card relative flex flex-col justify-between aspect-square"
      >
        <i
          className={classNames(
            alpha2country,
            "flag",
            "absolute right-6 top-6"
          )}
        />
      </figure>
      <figcaption className="px-2">
        <h3>{name}</h3>
        <small>{rank === 9999 ? "Unranked" : `Overall Rank: #${rank}`}</small>
      </figcaption>
    </Link>
  );
};

export default PilotCard;
