import classNames from "classnames";
import { alpha3ToAlpha2 } from "i18n-iso-countries";
import Link from "next/link";
import useSWR from "swr";

import { API_URL } from "@/constants";
import { components } from "@/types";
import { capitalise } from "@/utils/capitalise";
import { fetcher } from "@/utils/fetcher";

import FetchError from "../ui/fetchError";
import FetchLoading from "../ui/fetchLoading";

interface Props {
  judge: components["schemas"]["Judge"];
}

type Pilot = components["schemas"]["Pilot"];

const JudgeCard = ({ judge }: Props) => {
  const { _id, civlid, name, country, level } = judge;
  const urlName = name.toLowerCase().replace(/\s/g, "-");
  const alpha2country = alpha3ToAlpha2(country?.toUpperCase())?.toLowerCase();

  const { data: pilot, error } = useSWR<Pilot, Error>(
    civlid ? `${API_URL}/pilots/${civlid}` : null,
    fetcher
  );

  if (civlid && error) return <FetchError />;
  if (civlid && !pilot) return <FetchLoading />;

  const photo = pilot?.photo;

  return (
    <Link
      key={_id}
      title={`See ${name}'s profile`}
      href={`/pilots/${civlid}/${urlName}`}
      className={classNames(
        "flex flex-col rounded-xl pb-4",
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
      <figcaption className="mt-4 px-4 w-max self-center">
        <h3 className="text-sm text-left">{name}</h3>
        <small>{capitalise(level)}</small>
      </figcaption>
    </Link>
  );
};

export default JudgeCard;
