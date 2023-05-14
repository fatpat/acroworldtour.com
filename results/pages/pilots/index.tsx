import classNames from "classnames";
import { alpha3ToAlpha2 } from "i18n-iso-countries";
import Link from "next/link";
import useSWR from "swr";

import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Pilot = components["schemas"]["Pilot"];

const pageTitle = "Acro World Tour | Pilots";
const pageDescription =
  "Acro World Tour Pilots page. In this page you will find all the pilots that are part of the Acro World Tour.";
const headerTitle = "Pilots";
const headerSubtitle = "Acro World Tour";

const Pilots = () => {
  const { data: pilots, error } = useSWR<Pilot[]>(
    "https://api-preprod.acroworldtour.com/public/pilots",
    fetcher
  );

  if (error) return <div>Failed to load</div>;
  if (!pilots) return <div>Loading...</div>;

  return (
    <section className="mt-4 flex flex-wrap justify-start gap-x-8 gap-y-4">
      {pilots.map((pilot) => {
        const { _id: id, name, photo, country, rank } = pilot;
        const slug = name.toLowerCase().replace(/\s/g, "-");
        const alpha2country = alpha3ToAlpha2(
          country.toUpperCase()
        ).toLowerCase();
        return (
          <Link
            key={id}
            href={`/pilots/${id}/${slug}`}
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
            <small>
              {rank === 9999 ? "Unranked" : `Overall Rank: #${rank}`}
            </small>
          </Link>
        );
      })}
    </section>
  );
};

Pilots.pageTitle = pageTitle;
Pilots.pageDescription = pageDescription;
Pilots.headerTitle = headerTitle;
Pilots.headerSubtitle = headerSubtitle;

export default Pilots;
