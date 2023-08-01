import classNames from "classnames";
import { alpha3ToAlpha2 } from "i18n-iso-countries";
import Link from "next/link";

import { components } from "@/types";

interface Props {
  pilot: components["schemas"]["Pilot"];
  small?: boolean;
}

const PilotCard = ({ pilot, small }: Props) => {
  const { civlid, name, photo, country, rank } = pilot;
  const urlName = name.toLowerCase().replace(/\s/g, "-");
  const alpha2country = alpha3ToAlpha2(country?.toUpperCase())?.toLowerCase();

  return (
    <Link
      key={civlid}
      title={`See ${name}'s profile`}
      href={`/pilots/${civlid}/${urlName}`}
      className={classNames(
        "flex flex-col rounded-xl",
        "hover:-translate-y-2 hover:shadow-md",
        small ? "w-24 pb-2" : "w-48 pb-4",
      )}
    >
      <figure
        style={{ backgroundImage: `url('${photo}')` }}
        className="pilot-card relative flex aspect-square flex-col justify-between"
      >
        <i
          className={classNames(
            country && alpha2country,
            "flag",
            "absolute right-4 top-4",
          )}
        />
      </figure>
      <figcaption className="self-center pt-3">
        <h3 className="text-left">
          {name} {["🥇", "🥈", "🥉"][rank - 1]}
        </h3>
        {!small && <p>FAI Rank: #{rank}</p>}
      </figcaption>
    </Link>
  );
};

export default PilotCard;
