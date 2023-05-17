import classNames from "classnames";
import { alpha3ToAlpha2 } from "i18n-iso-countries";
import Link from "next/link";

import { components } from "@/types";

interface Props {
  pilot: components["schemas"]["Pilot"];
}

const PilotDetails = ({ pilot }: Props) => {
  const {
    civlid,
    name,
    link: civlLink,
    photo,
    country,
    rank,
    about,
    background_picture: backgroundPicture,
    links: socialLinks,
    sponsors,
  } = pilot;
  const alpha2country = alpha3ToAlpha2(country.toUpperCase()).toLowerCase();
  return (
    <>
      <Link
        href={photo}
        target="_blank"
        style={{ backgroundImage: `url('${photo}')` }}
        className="relative h-96 w-96 rounded-xl bg-cover bg-center bg-no-repeat"
      >
        <i
          className={classNames(
            alpha2country,
            "flag",
            "absolute right-6 top-6"
          )}
        />
      </Link>
      <article className="w-96">
        <h3 className="">{name}</h3>
        <small>{rank === 9999 ? "Unranked" : `Overall Rank: #${rank}`}</small>
      </article>
    </>
  );
};

export default PilotDetails;
