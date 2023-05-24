import classNames from "classnames";
import { alpha3ToAlpha2 } from "i18n-iso-countries";
import Link from "next/link";
import useSWR from "swr";

import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

interface Props {
  judge: components["schemas"]["Judge"];
}

type Pilot = components["schemas"]["Pilot"];

const JudgeCard = ({ judge }: Props) => {
  const {
    civlid,
    name,
    // photo_highres: photo,
    country,
    level,
  } = judge;
  const urlName = name.toLowerCase().replace(/\s/g, "-");
  const alpha2country = alpha3ToAlpha2(country?.toUpperCase())?.toLowerCase();

  const { data: pilot, error } = useSWR<Pilot, Error>(
    civlid ? `${API_URL}/pilots/${civlid}` : null,
    fetcher
  );

  const photo = pilot?.photo_highres;
  // const photo = "/martin-wyall-RYAUYkia-cI-unsplash.jpg";

  return (
    <Link
      key={civlid}
      title={`See ${name}'s profile`}
      href={`/pilots/${civlid}/${urlName}`}
      className={classNames(
        "flex w-full min-w-[200px] max-w-xs flex-col rounded-xl pb-4",
        "hover:-translate-y-2 hover:shadow-xl",
        !civlid && "pointer-events-none"
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
            "absolute right-1 top-3"
          )}
        />
      </figure>
      <figcaption className="px-2">
        <h3 className="text-sm">{name}</h3>
        <small>{`Level: #${level}`}</small>
      </figcaption>
    </Link>
  );
};

export default JudgeCard;
