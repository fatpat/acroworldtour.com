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
}

type Pilot = components["schemas"]["Pilot"];

const JudgeCard = ({ judge }: Props) => {
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
        "flex w-28 flex-col items-center rounded-xl pb-4",
        "hover:-translate-y-2 hover:shadow-xl",
        !civlid && "pointer-events-none",
      )}
    >
      <figure
        style={{ backgroundImage: `url('${photo}')` }}
        className="relative flex aspect-square w-28 flex-col justify-between rounded-xl bg-cover bg-center bg-no-repeat shadow shadow-awt-dark-400"
      >
        <i
          className={classNames(
            country && alpha2country,
            "flag",
            "absolute right-1 top-3",
          )}
        />
      </figure>
      <figcaption className="mt-4 self-center">
        <h6 className="text-sm">{name}</h6>
        <p className="text-center font-normal capitalize">{level}</p>
      </figcaption>
    </Link>
  );
};

export default JudgeCard;
