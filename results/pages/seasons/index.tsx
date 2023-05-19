import classNames from "classnames";
import useSWR from "swr";

import SeasonCard from "@/components/season/seasonCard";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Season = components["schemas"]["SeasonExport"];

const Competitions = () => {
  const { data: seasons, error: seasonsError } = useSWR<Season[]>(
    `${API_URL}/seasons`,
    fetcher
  );

  if (seasonsError) return <FetchError />;
  if (!seasons) return <FetchLoading />;

  const soloSeasons = seasons.filter((season) => season.type === "solo");

  return (
    <>
      <h2 className="mb-8">All Seasons</h2>
      <section className={classNames("wrapper")}>
        {soloSeasons.map((season) => {
          const {
            code,
            competitions,
            name,
            year,
            image,
            results,
            number_of_pilots,
          } = season;
          return <SeasonCard key={code} season={season} />;
        })}
      </section>
    </>
  );
};

export default Competitions;
