import classNames from "classnames";
import { alpha3ToAlpha2 } from "i18n-iso-countries";
import Link from "next/link";
import useSWR from "swr";

import { API_URL } from "@/constants";
import { components } from "@/types";

import FetchError from "../ui/fetchError";
import FetchLoading from "../ui/fetchLoading";

interface Props {
  judge: components["schemas"]["Judge"];
  small?: boolean;
}

type Pilot = components["schemas"]["Pilot"];

const JudgeCard = ({ judge, small }: Props) => {
  const { _id, civlid, name, country, level } = judge;
  const urlName = name.toLowerCase().replace(/\s/g, "-");
  const alpha2country = alpha3ToAlpha2(country?.toUpperCase())?.toLowerCase();

  const {
    data: pilot,
    error,
    isLoading,
  } = useSWR<Pilot, Error>(civlid ? `${API_URL}/pilots/${civlid}` : null);

  if (civlid && isLoading) return <FetchLoading />;
  if (civlid && error) return <FetchError />;
  if (civlid && !pilot) return <h2>Pilot not found</h2>;

  const photo = pilot?.photo;

  return (
    <Link
      key={_id}
      title={`See ${name}'s profile`}
      href={`/pilots/${civlid}/${urlName}`}
      className={classNames(
        "flex max-w-min flex-shrink flex-col items-center rounded-xl",
        "hover:-translate-y-1 hover:drop-shadow-md",
        !civlid && "pointer-events-none",
        small ? "pb-2" : "pb-4",
      )}
    >
      <figure
        style={{ backgroundImage: `url('${photo}')` }}
        className={classNames(
          "relative flex aspect-square flex-col justify-between",
          "bg-cover bg-center bg-no-repeat",
          "rounded-xl shadow shadow-awt-dark-400",
          small ? "mb-2 h-24" : "mb-4 h-48",
        )}
      >
        <i
          className={classNames(
            country && alpha2country,
            "flag",
            "absolute right-1 top-3",
          )}
        />
      </figure>
      <figcaption className="self-center px-2">
        <h6 className="text-sm">{name}</h6>
        <p className="text-center font-normal capitalize">{level}</p>
      </figcaption>
    </Link>
  );
};

export default JudgeCard;
