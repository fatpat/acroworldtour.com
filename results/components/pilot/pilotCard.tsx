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
      href={`/pilots/${civlid}/${urlName}`}
      className="flex flex-col pb-4"
    >
      <article
        style={{ backgroundImage: `url('${photo}')` }}
        className="relative flex h-64 w-64
         max-w-lg flex-col justify-between rounded-xl bg-cover bg-center bg-no-repeat p-4"
      >
        <i
          className={classNames(
            alpha2country,
            "flag",
            "absolute right-2 top-2"
          )}
        />
      </article>
      <h3 className="">{name}</h3>
      <small>{rank === 9999 ? "Unranked" : `Overall Rank: #${rank}`}</small>
    </Link>
  );
};

export default PilotCard;
