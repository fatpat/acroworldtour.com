import classNames from "classnames";
import { alpha3ToAlpha2 } from "i18n-iso-countries";
import Link from "next/link";

import { components } from "@/types";

interface Props {
  pilot: components["schemas"]["Pilot"];
}

const Pilot: React.FC<Props> = ({ pilot }) => {
  const { civlid, name, photo, country, rank } = pilot;
  const slug = name.toLowerCase().replace(/\s/g, "-");
  const alpha2country = alpha3ToAlpha2(country.toUpperCase()).toLowerCase();
  return (
    <Link
      key={civlid}
      href={`/pilots/${civlid}/${slug}`}
      className="flex flex-col pb-4"
    >
      <div
        style={{ backgroundImage: `url('${photo}')` }}
        className="relative mb-1 h-52 w-52 rounded-xl bg-cover bg-center bg-no-repeat"
      >
        <i
          className={classNames(
            alpha2country,
            "flag",
            "absolute right-2 top-2"
          )}
        />
      </div>
      <h3 className="">{name}</h3>
      <small>{rank === 9999 ? "Unranked" : `Overall Rank: #${rank}`}</small>
    </Link>
  );
};

export default Pilot;
